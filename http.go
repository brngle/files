package files

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/alioygur/gores"
	goalone "github.com/bwmarrin/go-alone"
	"github.com/dustin/go-humanize"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/sessions"
)

var mediaTags = map[string][]string{
	"application/sql":           {"text"},
	"application/x-shellscript": {"text"},
	"application/x-ruby":        {"text"},
	"application/x-yaml":        {"text"},
}

func getMediaTags(mediatype string) []string {
	add := []string{}
	if strings.HasPrefix(mediatype, "image/") {
		add = append(add, "image")
	}
	if strings.HasPrefix(mediatype, "text/") {
		add = append(add, "text")
	}
	if strings.HasPrefix(mediatype, "audio/") {
		add = append(add, "audio")
	}
	if strings.HasPrefix(mediatype, "video/") {
		add = append(add, "video")
	}

	res, ok := mediaTags[mediatype]
	if !ok {
		return add
	}
	return append(add, res...)
}

func hasMediaTag(mediaType, tag string) bool {
	tags := getMediaTags(mediaType)
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

func getTemplatePaths() []string {
	var result []string
	fs.WalkDir(templates, ".", func(path string, dir fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if filepath.Ext(dir.Name()) == ".html" {
			result = append(result, path)
		}
		return nil
	})
	return result
}

type HTTPService struct {
	done chan struct{}

	signer       *goalone.Sword
	sessionStore *sessions.CookieStore
	fileStore    *FileStore
	config       *Config
}

func NewHTTPService(config *Config, fileStore *FileStore) *HTTPService {
	return &HTTPService{
		signer:       goalone.New([]byte(config.HTTP.Secret)),
		sessionStore: sessions.NewCookieStore([]byte(config.HTTP.Secret)),
		fileStore:    fileStore,
		config:       config,
		done:         make(chan struct{}),
	}
}

func (h *HTTPService) getSession(w http.ResponseWriter, r *http.Request) *sessions.Session {
	session, err := h.sessionStore.Get(r, "session")
	if err != nil {
		gores.Error(w, http.StatusInternalServerError, "invalid or corrupted session")
		return nil
	}
	return session
}

func (h *HTTPService) withUser(w http.ResponseWriter, r *http.Request, must bool) string {
	authParts := strings.Split(r.Header.Get("Authorization"), " ")
	if len(authParts) > 1 && authParts[0] == "Token" {
		userId := h.validateToken(authParts[1])
		if userId != "" {
			return userId
		}
	}

	session := h.getSession(w, r)
	if session == nil {
		return ""
	}

	var discordUserId string = "0"
	raw, ok := session.Values["discord-user-id"]
	if raw != nil && ok {
		discordUserId = raw.(string)
	}

	if discordUserId == "" && must {
		gores.Error(w, http.StatusUnauthorized, "unauthorized")
		return ""
	}

	return discordUserId
}

func (h *HTTPService) isAdmin(userId string) bool {
	for _, role := range h.config.Roles {
		if role.HasUserId(userId) && role.Admin {
			return true
		}
	}
	return false
}

func (h *HTTPService) genToken(userId string) string {
	data, err := json.Marshal(userId)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(h.signer.Sign(data))
}

func (h *HTTPService) validateToken(token string) string {
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return ""
	}
	raw, err := h.signer.Unsign(decoded)
	if err != nil {
		return ""
	}
	var result string
	err = json.Unmarshal(raw, &result)
	if err != nil {
		return ""
	}
	return result
}

func (h *HTTPService) Stop() {
	close(h.done)
}

func (h *HTTPService) router() http.Handler {
	rtr := chi.NewRouter()
	rtr.Use(middleware.RealIP)
	rtr.Use(middleware.Logger)
	rtr.Use(middleware.Recoverer)

	rtr.Get("/", h.routeGetIndex)
	rtr.Get("/token", h.routeGetToken)
	rtr.Get("/user", h.routeGetUser)

	// discord stuff
	if h.config.Discord != nil {
		setupDiscord(h.config.Discord, h.config.HTTP.URL)
		rtr.Get("/discord/logout", h.discordGetLogoutRoute)
		rtr.Get("/discord/login", h.discordGetLoginRoute)
		rtr.Get("/discord/login/callback", h.discordGetLoginCallbackRoute)
	}

	rtr.Get("/static/*", h.routeGetStatic)

	rtr.Get("/s/{shareCode}", h.routeGetShareCode)

	rtr.Get("/volume/{volumeName}/upload", h.routeGetUpload)
	rtr.Post("/volume/{volumeName}/upload", h.routePostUpload)

	rtr.Get("/volume/{volumeName}/browse/*", h.routeGetVolume)
	rtr.Post("/volume/{volumeName}/share/*", h.routePostShareVolume)
	rtr.Post("/volume/{volumeName}/sharex", h.routePostSharex)
	rtr.Get("/volume/{volumeName}/search", h.routeGetSearch)
	rtr.Post("/volume/{volumeName}/search", h.routePostSearch)

	return rtr
}

func (h *HTTPService) Serve(ctx context.Context) error {
	srv := &http.Server{
		Addr:    h.config.HTTP.Bind,
		Handler: h.router(),
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// TODO: help
			log.Fatalf("listen: %s\n", err)
		}
	}()

	<-ctx.Done()
	log.Printf("shutting down server")
	return srv.Shutdown(context.Background())
}

func (h *HTTPService) routeGetIndex(w http.ResponseWriter, r *http.Request) {
	userId := h.withUser(w, r, false)
	if userId == "" {
		return
	}

	volumes := []*Volume{}
	for _, volume := range h.fileStore.Volumes {
		if volume.Privacy != "public" && userId == "0" {
			continue
		}

		// only admins can see unlisted volumes
		if volume.Privacy == "unlisted" && !h.isAdmin(userId) {
			continue
		}

		if volume.Privacy == "private" && !volume.HasUserId(userId) && !h.isAdmin(userId) {
			continue
		}

		volumes = append(volumes, volume)
	}

	sort.Slice(volumes, func(i, j int) bool {
		ii := volumes[i]
		jj := volumes[j]

		return ii.Name < jj.Name
	})

	h.template(w, "static/index.html", map[string]interface{}{
		"Volumes": volumes,
	})
}

func (h *HTTPService) routeGetToken(w http.ResponseWriter, r *http.Request) {
	userId := h.withUser(w, r, false)
	if userId == "" {
		return
	}

	if userId == "0" {
		gores.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	token := h.genToken(userId)
	h.template(w, "static/token.html", token)
}

func (h *HTTPService) routeGetUser(w http.ResponseWriter, r *http.Request) {
	userId := h.withUser(w, r, false)
	if userId == "" {
		return
	}

	h.templateFragment(w, "user-topbar", userId)
}

func (h *HTTPService) routeGetShareCode(w http.ResponseWriter, r *http.Request) {
	shareCode, err := GetShareCode(chi.URLParam(r, "shareCode"))
	if err != nil {
		gores.Error(w, http.StatusNotFound, "not found")
		return
	}

	v := r.URL.Query()
	v.Set("sc", shareCode.Code())

	http.Redirect(w, r, fmt.Sprintf("/volume/%s/browse/%s?%s", shareCode.Volume, shareCode.Path, v.Encode()), http.StatusTemporaryRedirect)
}

func (h *HTTPService) routePostShareVolume(w http.ResponseWriter, r *http.Request) {
	volume, ok := h.fileStore.Volumes[chi.URLParam(r, "volumeName")]
	if !ok {
		gores.Error(w, http.StatusNotFound, "not found")
		return
	}

	userId := h.withUser(w, r, true)
	if userId == "" {
		return
	}

	path, err := url.PathUnescape(chi.URLParam(r, "*"))
	if err != nil {
		panic(err)
	}

	if volume.Privacy == "private" && !volume.HasUserId(userId) && !h.isAdmin(userId) {
		gores.Error(w, http.StatusNotFound, "not found")
		return
	}

	// make sure the path exists
	_, err = volume.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			gores.Error(w, http.StatusNotFound, "not found")
			return
		}

		log.Printf("err = %v", err)
		gores.Error(w, http.StatusInternalServerError, "failed to stat path")
		return
	}

	shareCode, err := MakeShareCode(volume.Name, path)
	if err != nil {
		gores.Error(w, http.StatusInternalServerError, "failed to generate share code")
		return
	}

	url := h.config.HTTP.ShareURL
	if url != "" {
		url = fmt.Sprintf(url, shareCode.Code())
	} else {
		url = shareCode.URL(h.config.HTTP.URL)
	}

	h.templateFragment(w, "share-code", url)
	return
}

func (h *HTTPService) routeGetVolume(w http.ResponseWriter, r *http.Request) {
	volume, ok := h.fileStore.Volumes[chi.URLParam(r, "volumeName")]
	if !ok {
		gores.Error(w, http.StatusNotFound, "not found")
		return
	}

	userId := h.withUser(w, r, false)
	if userId == "" {
		return
	}

	path, err := url.PathUnescape(chi.URLParam(r, "*"))
	if err != nil {
		panic(err)
	}

	var shareCode *ShareCode
	shareCodeRaw := r.URL.Query().Get("sc")
	if shareCodeRaw != "" {
		it, err := GetShareCode(shareCodeRaw)
		if err != nil {
			gores.Error(w, http.StatusBadRequest, "invalid share code")
			return
		}

		if it.Volume != volume.Name || !strings.HasPrefix(path, it.Path) {
			gores.Error(w, http.StatusBadRequest, "invalid share path")
			return
		}

		shareCode = it
	}

	if volume.Privacy == "private" && !volume.HasUserId(userId) && !h.isAdmin(userId) && shareCode == nil {
		gores.Error(w, http.StatusNotFound, "not found")
		return
	}

	canList := true
	if volume.Privacy == "unlisted" && !volume.HasUserId(userId) && !h.isAdmin(userId) && shareCode == nil {
		canList = false
	}

	h.servePath(w, r, volume, path, canList, shareCode)
}

func (h *HTTPService) servePath(w http.ResponseWriter, r *http.Request, volume *Volume, path string, canList bool, shareCode *ShareCode) {
	info, err := volume.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			gores.Error(w, http.StatusNotFound, "not found")
			return
		}

		log.Printf("err = %v", err)
		gores.Error(w, http.StatusInternalServerError, "failed to stat path")
		return
	}

	if info.IsDir() && !canList {
		gores.Error(w, http.StatusNotFound, "not found")
		return
	}

	download := r.URL.Query().Has("download")
	raw := r.URL.Query().Has("raw")

	if raw || download {
		if info.IsDir() {
			gores.Error(w, http.StatusBadRequest, "cannot view or download directory")
			return
		}

		f, err := volume.Open(path)
		if err != nil {
			gores.Error(w, http.StatusInternalServerError, "failed to open file for download")
			return
		}
		defer f.Close()
		if download {
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", info.Name()))
		}
		http.ServeContent(w, r, info.Name(), info.ModTime(), f)
		return
	}

	compress := r.URL.Query().Get("compress")
	if compress != "" && !volume.HasFeature("compress") {
		gores.Error(w, http.StatusBadRequest, "compression is not available")
		return
	}

	var entries []*VolumeEntry
	var mimetype string
	var content string

	if info.IsDir() {
		switch compress {
		case "zip":
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.zip\"", filepath.Base(path)))

			zw := zip.NewWriter(w)
			err := volume.WalkDir(path, func(s string, de fs.DirEntry, err error) error {
				if de.IsDir() {
					return nil
				}

				f, err := zw.Create(strings.TrimPrefix(s, path+"/"))
				if err != nil {
					return err
				}

				data, err := volume.Data(s)
				if err != nil {
					return err
				}

				_, err = f.Write(data)
				if err != nil {
					return err
				}

				return nil
			})
			if err != nil {
				log.Printf("err = %v", err)
				panic(err)
			}

			err = zw.Close()
			if err != nil {
				panic(err)
			}

			return
		case "":
			break
		default:
			gores.Error(w, http.StatusBadRequest, "unsupported compression type")
			return
		}

		entries, err = volume.Entries(path)
		if err != nil {
			gores.Error(w, http.StatusInternalServerError, "failed to list directory")
			return
		}

		if len(entries) > 1000 {
			entries = entries[:1000]
		}
	} else {
		mtraw := mime.TypeByExtension(filepath.Ext(path))
		mimetype, _, err = mime.ParseMediaType(mtraw)
		if err == nil {
			if hasMediaTag(mimetype, "text") && ByteSize(info.Size()) < 8*MB {
				data, err := volume.Data(path)
				if err == nil {
					content = string(data)
				}
			}
		}
	}

	args := url.Values{}

	if shareCode != nil {
		args.Add("sc", shareCode.Code())
	}

	link := fmt.Sprintf("/volume/%s/browse/%s?%s", volume.Name, path, args.Encode())

	template := "static/volume.html"

	h.template(w, template, map[string]interface{}{
		"Gallery":   r.URL.Query().Has("gallery") && info.IsDir(),
		"Volume":    volume,
		"Path":      path,
		"Dir":       filepath.Dir(path),
		"Stat":      info,
		"Entries":   entries,
		"Type":      mimetype,
		"Content":   content,
		"HumanSize": humanize.Bytes(uint64(info.Size())),
		"HasTag": func(tag string) bool {
			return hasMediaTag(mimetype, tag)
		},
		"MakeLink": func(args ...string) string {
			if len(args) > 0 {
				return fmt.Sprintf("%s&%s", link, strings.Join(args, "&"))
			}
			return link
		},
	})
}

func (h *HTTPService) routeGetStatic(w http.ResponseWriter, r *http.Request) {
	param := chi.URLParam(r, "*")
	ext := filepath.Ext(param)
	if param == "" || (ext != ".js" && ext != ".css") {
		param = "index.html"
	}

	data, err := dist.ReadFile("dist/" + param)
	if err != nil {
		gores.Error(w, http.StatusNotFound, "Not Found")
		return
	}

	fileName := filepath.Base(param)
	http.ServeContent(w, r, fileName, time.Now(), bytes.NewReader(data))
}

func (h *HTTPService) template(w http.ResponseWriter, templateName string, context interface{}) {
	paths := getTemplatePaths()
	paths = append(paths, templateName)

	ts, err := template.ParseFS(templates, paths...)
	if err != nil {
		gores.Error(w, 500, fmt.Sprintf("Error rendering template: %v", err))
		return
	}

	err = ts.Execute(w, context)
	if err != nil {
		gores.Error(w, 500, fmt.Sprintf("Error rendering template: %v", err))
	}
}

func (h *HTTPService) templateFragment(w http.ResponseWriter, fragName string, context interface{}) {
	paths := getTemplatePaths()

	ts, err := template.ParseFS(templates, paths...)
	if err != nil {
		gores.Error(w, 500, fmt.Sprintf("Error rendering template: %v", err))
		return
	}

	err = ts.ExecuteTemplate(w, fragName, context)
	if err != nil {
		gores.Error(w, 500, fmt.Sprintf("Error rendering template: %v", err))
	}
}
