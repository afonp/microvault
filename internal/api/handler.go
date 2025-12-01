package api

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/afonp/microvault/internal/db"
	"github.com/afonp/microvault/internal/hashing"
)

type Handler struct {
	store  *db.Store
	ring   *hashing.Ring
	client *http.Client
}

func NewHandler(store *db.Store, ring *hashing.Ring) *Handler {
	return &Handler{
		store:  store,
		ring:   ring,
		client: &http.Client{},
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
	// use consistent hashing to pick volume
	targetVolume := h.ring.GetNode(key)
	if targetVolume == "" {
		http.Error(w, "no volumes available", http.StatusServiceUnavailable)
		return
	}

	// we PUT to the volume root, and it returns the hash
	targetURL := targetVolume

	req, err := http.NewRequest(http.MethodPut, targetURL, r.Body)
	if err != nil {
		http.Error(w, "failed to create request", http.StatusInternalServerError)
		return
	}
	req.ContentLength = r.ContentLength

	resp, err := h.client.Do(req)
	if err != nil {
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
		return
	}

	// read hash from response body
	hashBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "failed to read upstream response", http.StatusBadGateway)
		return
	}
	hash := string(hashBytes)

	// success, update db
	// store the full URL to the blob: volumeURL + "/" + sharded_path
	// volume stores as /ab/cd/hash
	if len(hash) < 4 {
		http.Error(w, "invalid hash received", http.StatusBadGateway)
		return
	}
	blobURL := fmt.Sprintf("%s/%s/%s/%s", targetVolume, hash[:2], hash[2:4], hash)

	if err := h.store.PutBlob(key, blobURL); err != nil {
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
	targetURL := volumeID
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
