package files

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/alioygur/gores"
	"github.com/go-chi/chi/v5"
)

func (h *HTTPService) routePostSharex(w http.ResponseWriter, r *http.Request) {
	volume, ok := h.fileStore.Volumes[chi.URLParam(r, "volumeName")]
	if !ok {
		gores.Error(w, http.StatusNotFound, "not found")
		return
	}

	userId := h.withUser(w, r, false)
	if userId == "" {
		return
	}

	if volume.Privacy != "public" && !volume.HasUserId(userId) && !h.isAdmin(userId) {
		gores.Error(w, http.StatusNotFound, "not found")
		return
	}

	file, fileHeader, err := r.FormFile("image")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	path := filepath.Join(userId, fileHeader.Filename)

	err = volume.MkdirAll(userId, os.ModePerm)
	if err != nil {
		gores.Error(w, http.StatusInternalServerError, "failed to create user directory")
		return

	}

	dst, err := volume.Create(path)
	if err != nil {
		gores.Error(w, http.StatusInternalServerError, "failed to create file")
		return
	}
	defer dst.Close()

	_, err = io.Copy(dst, file)
	if err != nil {
		gores.Error(w, http.StatusInternalServerError, "failed to create file")
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

	gores.JSON(w, http.StatusOK, map[string]string{
		"link": url,
	})
}
