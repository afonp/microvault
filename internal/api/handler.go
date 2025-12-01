package api

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/afonp/microvault/internal/db"
	"github.com/afonp/microvault/internal/hashing"
)

type Handler struct {
	store    *db.Store
	ring     *hashing.Ring
	client   *http.Client
	replicas int
}

func NewHandler(store *db.Store, ring *hashing.Ring, replicas int) *Handler {
	return &Handler{
		store:    store,
		ring:     ring,
		client:   &http.Client{Timeout: 5 * time.Second},
		replicas: replicas,
	}
}

// serve_blob handles GET requests
// redirects to one of the volume servers
func (h *Handler) ServeBlob(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/blob/")
	if key == "" {
		http.Error(w, "missing key", http.StatusBadRequest)
		return
	}

	volumeIDs, err := h.store.GetBlob(key)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if len(volumeIDs) == 0 {
		http.NotFound(w, r)
		return
	}

	// redirect to a random replica for load balancing
	target := volumeIDs[rand.Intn(len(volumeIDs))]
	http.Redirect(w, r, target, http.StatusFound)
}

// put_blob handles PUT requests
// proxies data to the volume servers (N replicas) and updates metadata
func (h *Handler) PutBlob(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/blob/")
	if key == "" {
		http.Error(w, "missing key", http.StatusBadRequest)
		return
	}

	// read body once so we can replay it
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusInternalServerError)
		return
	}

	// use consistent hashing to pick volumes
	targetVolumes := h.ring.GetNodes(key, h.replicas)
	if len(targetVolumes) == 0 {
		http.Error(w, "no volumes available", http.StatusServiceUnavailable)
		return
	}

	var blobURLs []string
	var mu sync.Mutex
	var wg sync.WaitGroup
	errChan := make(chan error, len(targetVolumes))

	// write to all replicas in parallel
	for _, vol := range targetVolumes {
		wg.Add(1)
		go func(v string) {
			defer wg.Done()

			// we PUT to the volume root, and it returns the hash
			req, err := http.NewRequest(http.MethodPut, v, strings.NewReader(string(bodyBytes)))
			if err != nil {
				errChan <- err
				return
			}
			req.ContentLength = int64(len(bodyBytes))

			resp, err := h.client.Do(req)
			if err != nil {
				errChan <- err
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode >= 300 {
				errChan <- fmt.Errorf("upstream error: %d", resp.StatusCode)
				return
			}

			// read hash from response body
			hashBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				errChan <- err
				return
			}
			hash := string(hashBytes)
			if len(hash) < 4 {
				errChan <- fmt.Errorf("invalid hash")
				return
			}

			// construct blob URL
			// volume stores as /ab/cd/hash
			u := fmt.Sprintf("%s/%s/%s/%s", v, hash[:2], hash[2:4], hash)

			mu.Lock()
			blobURLs = append(blobURLs, u)
			mu.Unlock()
		}(vol)
	}

	wg.Wait()
	close(errChan)

	// check for errors
	// for strict consistency, if any fail, we might want to fail the whole thing.
	// but for now, let's say if we wrote to at least 1, we are okay?

	if len(blobURLs) != len(targetVolumes) {
		// some failed
		// rollback in the future
		http.Error(w, "failed to write to all replicas", http.StatusBadGateway)
		return
	}

	// success, update db
	if err := h.store.PutBlob(key, blobURLs); err != nil {
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

	volumeIDs, err := h.store.GetBlob(key)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if len(volumeIDs) == 0 {
		http.NotFound(w, r)
		return
	}

	// delete from all replicas
	var wg sync.WaitGroup
	for _, url := range volumeIDs {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			req, _ := http.NewRequest(http.MethodDelete, u, nil)
			h.client.Do(req)
		}(url)
	}
	wg.Wait()

	// remove from db
	if err := h.store.DeleteBlob(key); err != nil {
		http.Error(w, "failed to update index", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
