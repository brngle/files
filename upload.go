package files

import (
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/alioygur/gores"
	"github.com/go-chi/chi/v5"
)

func (h *HTTPService) routeGetUpload(w http.ResponseWriter, r *http.Request) {
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

	if !volume.HasFeature("upload") {
		gores.Error(w, http.StatusNotFound, "upload is not available for this volume")
		return
	}

	path := r.URL.Query().Get("path")

	h.template(w, "static/upload.html", map[string]interface{}{
		"Volume": volume,
		"Path":   path,
	})
}

func (h *HTTPService) routePostUpload(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		gores.Error(w, http.StatusBadRequest, err.Error())
		return
	}

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

	if !volume.HasFeature("upload") {
		gores.Error(w, http.StatusNotFound, "upload is not available for this volume")
		return
	}

	path := r.FormValue("path")

	file, handler, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	f, err := volume.OpenFile(filepath.Join(path, handler.Filename), os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer f.Close()

	_, err = io.Copy(f, file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	gores.NoContent(w)
}
