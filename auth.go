package files

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/alioygur/gores"
	goalone "github.com/bwmarrin/go-alone"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/sessions"
)

type VolumeLease interface {
	Volume() Volume
	Read() bool
	Write() bool
	Search() bool
	List() bool
}

type Authorization interface {
	DiscordUserId() string
	CanAccess(volume *Volume, path string, full bool) bool
}

type UserAuthorization struct {
	id      string
	isAdmin bool
}

func NewUserAuthorization(userId string, isAdmin bool) *UserAuthorization {
	return &UserAuthorization{id: userId, isAdmin: isAdmin}
}

func (u *UserAuthorization) DiscordUserId() string {
	return u.id
}

func (u *UserAuthorization) CanAccess(volume *Volume, path string, full bool) bool {
	if u.isAdmin {
		return true
	}

	if volume.HasUserId(u.id) {
		return true
	}

	return false
}

type APIKeyAuthorization struct {
	key *APIKey
}

func NewAPIKeyAuthorization(key *APIKey) *APIKeyAuthorization {
	return &APIKeyAuthorization{key: key}
}

func (u *APIKeyAuthorization) DiscordUserId() string {
	return ""
}

func (u *APIKeyAuthorization) CanAccess(volume *Volume, path string, full bool) bool {
	config := u.key.Config.Data()
	if config.Volumes == nil || len(config.Volumes) == 0 {
		return true
	}

	for _, vol := range config.Volumes {
		if vol == volume.Name {
			return true
		}
	}

	return false
}

type ShareCodeAuthorization struct {
	shareCode *ShareCode
}

func NewShareCodeAuthorization(shareCode *ShareCode) *ShareCodeAuthorization {
	return &ShareCodeAuthorization{shareCode: shareCode}
}

func (u *ShareCodeAuthorization) DiscordUserId() string {
	return ""
}

func (u *ShareCodeAuthorization) CanAccess(volume *Volume, path string, full bool) bool {
	if !full && u.shareCode.Volume == volume.Name && strings.HasPrefix(path, u.shareCode.Path) {
		return true
	}

	return false
}

type AuthStore struct {
	signer       *goalone.Sword
	fileStore    *FileStore
	sessionStore *sessions.CookieStore
	config       *Config
}

func NewAuthStore(fileStore *FileStore, config *Config) *AuthStore {
	return &AuthStore{
		signer:       goalone.New([]byte(config.HTTP.Secret)),
		fileStore:    fileStore,
		sessionStore: sessions.NewCookieStore([]byte(config.HTTP.Secret)),
		config:       config,
	}
}

func (a *AuthStore) GetSession(r *http.Request) *sessions.Session {
	session, err := a.sessionStore.Get(r, "session")
	if err != nil {
		return nil
	}
	return session
}

func (a *AuthStore) Check(r *http.Request) Authorization {
	authParts := strings.Split(r.Header.Get("Authorization"), " ")

	if len(authParts) > 1 && strings.ToLower(authParts[0]) == "token" {
		userId := a.ValidateUserToken(authParts[1])
		if userId != "" {
			return NewUserAuthorization(userId, a.config.IsAdmin(userId))
		}
		return nil
	} else if len(authParts) > 1 && strings.ToLower(authParts[0]) == "apikey" {
		key, err := GetAPIKey(authParts[1])
		if err == nil {
			return NewAPIKeyAuthorization(key)
		}
		return nil
	} else if r.URL.Query().Get("sc") != "" {
		shareCodeRaw := r.URL.Query().Get("sc")
		if shareCodeRaw != "" {
			it, err := GetShareCode(shareCodeRaw)
			if err != nil {
				return nil
			}

			return NewShareCodeAuthorization(it)
		}
		return nil
	} else {
		session := a.GetSession(r)
		if session == nil {
			return nil
		}

		var discordUserId string = "0"
		raw, ok := session.Values["discord-user-id"]
		if raw != nil && ok {
			discordUserId = raw.(string)
		}

		if discordUserId == "" || discordUserId == "0" {
			return nil
		}

		return NewUserAuthorization(discordUserId, a.config.IsAdmin(discordUserId))
	}
}

type AuthReq = string

func (a *AuthStore) GetVolume(w http.ResponseWriter, r *http.Request, needAuth bool) (*Volume, Authorization) {
	auth := a.Check(r)
	volume, ok := a.fileStore.Volumes[chi.URLParam(r, "volumeName")]

	if !needAuth && (volume.Privacy == "public" || volume.Privacy == "unlisted") {
		return volume, auth
	}

	if auth == nil {
		gores.Error(w, http.StatusUnauthorized, "unauthorized")
		return nil, nil
	}

	// TODO: this being implicit is so fucking aids, just pass it in
	path, err := url.PathUnescape(chi.URLParam(r, "*"))
	if err != nil {
		panic(err)
	}

	if !ok || !auth.CanAccess(volume, path, needAuth) {
		gores.Error(w, http.StatusNotFound, "not found")
		return nil, nil
	}

	return volume, auth
}

func (a *AuthStore) GenerateUserToken(userId string) string {
	data, err := json.Marshal(userId)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(a.signer.Sign(data))
}

func (a *AuthStore) ValidateUserToken(token string) string {
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return ""
	}
	raw, err := a.signer.Unsign(decoded)
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
