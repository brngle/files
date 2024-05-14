package files

import (
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/alioygur/gores"
)

func (h *HTTPService) routePostSharex(w http.ResponseWriter, r *http.Request) {
	volume, auth := h.authStore.GetVolume(w, r, true)
	if volume == nil {
		return
	}

	file, fileHeader, err := r.FormFile("image")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	discordUserId := auth.DiscordUserId()
	if discordUserId == "" {
		gores.Error(w, http.StatusBadRequest, "must be authorized with a user account to use sharex")
		return
	}

	err = volume.MkdirAll(discordUserId, os.ModePerm)
	if err != nil {
		gores.Error(w, http.StatusInternalServerError, "failed to create user directory")
		return
	}

	path := filepath.Join(discordUserId, fileHeader.Filename)
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

	url := shareCode.URL(h.config.HTTP)
	gores.JSON(w, http.StatusOK, map[string]string{
		"link": url,
	})
}
