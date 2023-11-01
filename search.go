package files

import (
	"io/fs"
	"log"
	"net/http"

	"github.com/alioygur/gores"
	"github.com/charlievieth/fastwalk"
	"github.com/go-chi/chi/v5"
)

func (v *Volume) Search(p string, query string) error {
	path, err := v.path(p)
	if err != nil {
		return err
	}

	return fastwalk.Walk(&fastwalk.Config{Follow: false}, path, func(path string, d fs.DirEntry, err error) error {
		log.Printf("path = %v", path)
		return nil
	})
}

func (h *HTTPService) routePostSearch(w http.ResponseWriter, r *http.Request) {
	gores.HTML(w, http.StatusOK, "<div>YO</div>")
}

func (h *HTTPService) routeGetSearch(w http.ResponseWriter, r *http.Request) {
	volume, ok := h.fileStore.Volumes[chi.URLParam(r, "volumeName")]
	if !ok {
		gores.Error(w, http.StatusNotFound, "not found")
		return
	}

	userId := h.withUser(w, r, true)
	if userId == "" {
		return
	}

	if (volume.Privacy == "private" || volume.Privacy == "unlisted") && !volume.HasUserId(userId) && !h.isAdmin(userId) {
		gores.Error(w, http.StatusNotFound, "not found")
		return
	}

	h.template(w, "static/search.html", map[string]interface{}{
		"Volume": volume,
	})
}
