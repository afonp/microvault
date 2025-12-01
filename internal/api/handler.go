package api

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/afonp/microvault/internal/db"
)

type Handler struct {
	store     *db.Store
	volumeURL string // single volume for phase 1
	client    *http.Client
}

func NewHandler(store *db.Store, volumeURL string) *Handler {
	return &Handler{
		store:     store,
		volumeURL: volumeURL,
		client:    &http.Client{},
	}
}

// serve_blob handles GET requests
// redirects to the volume server
func (h *Handler) ServeBlob(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/blob/")
	if key == "" {
		http.Error(w, "missing key", http.StatusBadRequest)
		return
	}

	volumeID, err := h.store.GetBlob(key)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if volumeID == "" {
		http.NotFound(w, r)
		return
	}

	// redirect to volume
	// volumeID in phase 1 is just the url, or we map it.
	// let's assume volumeID stored in db is the base url for now, or we match it.
	// for simplicity in phase 1, we just use the stored value which we will save as the url.
	http.Redirect(w, r, fmt.Sprintf("%s/%s", volumeID, key), http.StatusFound)
}

// put_blob handles PUT requests
// proxies data to the volume server and updates metadata
func (h *Handler) PutBlob(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/blob/")
	if key == "" {
		http.Error(w, "missing key", http.StatusBadRequest)
		return
	}

	// proxy to volume
	// in phase 1, we always use h.volumeURL
	targetURL := fmt.Sprintf("%s/%s", h.volumeURL, key)

	req, err := http.NewRequest(http.MethodPut, targetURL, r.Body)
	if err != nil {
		http.Error(w, "failed to create request", http.StatusInternalServerError)
		return
	}
	// copy headers? usually good practice but maybe overkill for "radically simple"
	// let's just set content-length if known
	req.ContentLength = r.ContentLength

	resp, err := h.client.Do(req)
	if err != nil {
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		// forward error
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
		return
	}

	// success, update db
	if err := h.store.PutBlob(key, h.volumeURL); err != nil {
		// consistency issue here if db fails but write succeeded.
		// for "simple", we log and fail? or just fail.
		// user can retry.
		http.Error(w, "failed to update index", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// delete_blob handles DELETE requests
func (h *Handler) DeleteBlob(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/blob/")
	if key == "" {
		http.Error(w, "missing key", http.StatusBadRequest)
		return
	}

	volumeID, err := h.store.GetBlob(key)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if volumeID == "" {
		http.NotFound(w, r)
		return
	}

	// delete from volume
	targetURL := fmt.Sprintf("%s/%s", volumeID, key)
	req, err := http.NewRequest(http.MethodDelete, targetURL, nil)
	if err != nil {
		http.Error(w, "failed to create request", http.StatusInternalServerError)
		return
	}

	resp, err := h.client.Do(req)
	if err != nil {
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 && resp.StatusCode != 404 {
		w.WriteHeader(resp.StatusCode)
		return
	}

	// remove from db
	if err := h.store.DeleteBlob(key); err != nil {
		http.Error(w, "failed to update index", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
