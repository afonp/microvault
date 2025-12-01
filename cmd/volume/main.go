package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	port := flag.String("port", "8081", "port to listen on")
	rootDir := flag.String("root", "./data", "root directory for blob storage")
	flag.Parse()

	if err := os.MkdirAll(*rootDir, 0755); err != nil {
		log.Fatalf("failed to create root dir: %v", err)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// path should be /blob/{key} or just /{key} depending on how nginx routes it
		// let's assume nginx passes the full path /blob/{key} or rewrites it.
		// for this simple wrapper, let's assume we get /{key} or /blob/{key} and we just want the key.
		// actually, the master redirects to {volumeURL}/{key}.
		// so if volumeURL is http://localhost:8081, request is GET /key (served by nginx)
		// PUT /key (handled here).

		key := strings.TrimPrefix(r.URL.Path, "/")

		switch r.Method {
		case http.MethodPut:
			handle_put(w, r, *rootDir, key)
		case http.MethodDelete:
			if key == "" {
				http.Error(w, "missing key", http.StatusBadRequest)
				return
			}
			handle_delete(w, r, *rootDir, key)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	log.Printf("volume wrapper listening on :%s", *port)
	if err := http.ListenAndServe(":"+*port, nil); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func handle_put(w http.ResponseWriter, r *http.Request, root, key string) {
	// we want to store by content hash, but the key is user provided?
	// "Files named by content hash".
	// "Optional user-defined keys map to content hashes".
	// The Master stores Key -> VolumeID.
	// But wait, if the file is named by content hash, then the Volume Server needs to know the hash.
	// If the Master says "PUT /blob/user-key", and proxies to Volume "PUT /user-key",
	// The Volume Server should read the content, calculate hash, save file as "hash",
	// AND somehow we need to know that "user-key" maps to "hash"?
	//
	// Re-reading architecture:
	// "Master Server ... Metadata index (SQLite: key -> [volume_ids])"
	// "Volume Server ... files named by content hash"
	//
	// If files are named by content hash, then the Master needs to know the content hash to redirect GETs?
	// "GET /blob/{key} -> 302 to volume".
	// If Volume stores as /data/ab/cd/hash..., then the redirect URL must be .../hash.
	// So the Master needs to know the hash.
	//
	// This implies:
	// 1. Client PUTs to Master.
	// 2. Master reads stream, calculates hash, writes to Volume (as /hash or just PUT data and Volume returns hash?).
	// 3. Master updates DB: key -> volume_id + hash? Or just volume_id and the redirect URL includes hash?
	//
	// Actually, if "files named by content hash", then the Volume Server is a CAS (Content Addressable Storage).
	// The Master maps UserKey -> ContentHash (and VolumeID).
	//
	// Let's refine the Master implementation.
	// Current Master implementation:
	// PutBlob: proxies to volumeURL/key.
	// This means Volume receives "PUT /user-key".
	// If Volume saves it as "user-key", that's not "named by content hash".
	//
	// If Volume saves it as "hash", it must return the hash to the Master.
	//
	// Let's update Volume Server to:
	// 1. Receive PUT.
	// 2. Calculate Hash while writing to temp file.
	// 3. Move temp file to /data/ab/cd/hash.
	// 4. Return Hash in response (e.g. Header or Body).
	//
	// Then Master needs to update DB with (Key, VolumeID, Hash).
	//
	// Wait, the prompt says: "Metadata index ... key -> [volume_ids]". It doesn't explicitly say it stores the hash.
	// But "files named by content hash" is explicit for Volume Server.
	// If Master redirects to Volume, it must redirect to the file path.
	// So Master MUST store the hash (or the full path on volume).
	//
	// Let's assume Master stores `key -> volume_url + / + hash`.
	//
	// So, Volume Server `handle_put` should return the Hash.
	// Master `PutBlob` should read the Hash from Volume response and store it.
	//
	// Let's implement Volume Server to return the hash.

	tempFile, err := os.CreateTemp(root, "upload-*")
	if err != nil {
		http.Error(w, "failed to create temp file", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tempFile.Name()) // cleanup if not renamed

	hasher := sha256.New()
	writer := io.MultiWriter(tempFile, hasher)

	if _, err := io.Copy(writer, r.Body); err != nil {
		http.Error(w, "failed to write data", http.StatusInternalServerError)
		return
	}
	tempFile.Close()

	hash := hex.EncodeToString(hasher.Sum(nil))

	// create directory structure /ab/cd/
	dir := filepath.Join(root, hash[:2], hash[2:4])
	if err := os.MkdirAll(dir, 0755); err != nil {
		http.Error(w, "failed to create directory", http.StatusInternalServerError)
		return
	}

	finalPath := filepath.Join(dir, hash)
	if err := os.Rename(tempFile.Name(), finalPath); err != nil {
		// if rename fails (e.g. cross-device), copy and delete
		// for now assume same filesystem
		http.Error(w, "failed to save file", http.StatusInternalServerError)
		return
	}

	// return hash
	w.Header().Set("X-Content-Hash", hash)
	w.WriteHeader(http.StatusCreated)
	fmt.Fprint(w, hash)
}

func handle_delete(w http.ResponseWriter, r *http.Request, root, key string) {
	// extract hash from path (e.g. ab/cd/hash -> hash)
	hash := filepath.Base(key)

	// validate hash format (simple check)
	if len(hash) != 64 {
		http.Error(w, "invalid hash", http.StatusBadRequest)
		return
	}

	path := filepath.Join(root, hash[:2], hash[2:4], hash)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "failed to delete", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
