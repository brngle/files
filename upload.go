package files

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/alioygur/gores"
)

func (h *HTTPService) routeGetUpload(w http.ResponseWriter, r *http.Request) {
	volume, _ := h.authStore.GetVolume(w, r, true)
	if volume == nil {
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
	volume, _ := h.authStore.GetVolume(w, r, true)
	if volume == nil {
		return
	}

	err := r.ParseMultipartForm(256 << 20)
	if err != nil {
		gores.Error(w, http.StatusBadRequest, err.Error())
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

	url := fmt.Sprintf("%s/volume/%s/browse/%s", h.config.HTTP.BaseURL(), volume.Name, filepath.Join(path, handler.Filename))
	w.Header().Add("HX-Redirect", url)
	gores.NoContent(w)
}
