package files

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/alioygur/gores"
	"github.com/charlievieth/fastwalk"
	"github.com/lithammer/fuzzysearch/fuzzy"
)

var ErrMaxResults = errors.New("max results hit")

func (v *Volume) Search(rootPath string, query string, fuzz bool, maxResults int) ([]*VolumeEntry, error) {
	root, err := v.path(rootPath)
	if err != nil {
		return nil, err
	}

	results := []*VolumeEntry{}
	err = fastwalk.Walk(&fastwalk.Config{Follow: false}, root, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}

		name := filepath.Base(path)

		matches := false
		if fuzz {
			matches = fuzzy.Match(query, strings.ToLower(name))
		} else {
			matches = strings.Contains(strings.ToLower(name), query)
		}

		if !matches {
			return nil
		}

		p := filepath.Join(rootPath, strings.TrimPrefix(path, root))
		entry, err := v.Entry(p)
		if err != nil {
			return err
		}
		results = append(results, entry)
		if len(results) >= maxResults {
			return ErrMaxResults
		}
		return nil
	})
	if errors.Is(err, ErrMaxResults) {
		return results, nil
	}

	if err != nil {
		return nil, err
	}

	return results, nil
}

func (h *HTTPService) routePostSearch(w http.ResponseWriter, r *http.Request) {
	volume, _ := h.authStore.GetVolume(w, r, true)
	if volume == nil {
		return
	}

	if !volume.HasFeature("search") {
		gores.Error(w, http.StatusNotFound, "search is not available for this volume")
		return
	}

	path := r.URL.Query().Get("path")
	err := r.ParseForm()
	if err != nil {
		gores.Error(w, http.StatusBadRequest, "invalid form data")
		return
	}

	query := strings.ToLower(r.Form.Get("search"))
	results, err := volume.Search(path, query, r.URL.Query().Has("fuzzy"), 100)
	if err != nil {
		gores.HTML(w, http.StatusOK, fmt.Sprintf("<div>Failed to search: %s</div>", err))
		return
	}

	h.templateFragment(w, "search-results", map[string]interface{}{
		"Results": results,
		"Volume":  volume,
	})
}

func (h *HTTPService) routeGetSearch(w http.ResponseWriter, r *http.Request) {
	volume, _ := h.authStore.GetVolume(w, r, true)
	if volume == nil {
		return
	}

	if !volume.HasFeature("search") {
		gores.Error(w, http.StatusNotFound, "search is not available for this volume")
		return
	}

	path := r.URL.Query().Get("path")

	h.template(w, "static/search.html", map[string]interface{}{
		"Volume": volume,
		"Path":   path,
	})
}
