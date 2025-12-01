package tools

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func Compact(ctx *Context) error {
	store, err := ctx.GetStore()
	if err != nil {
		return err
	}
	defer store.Close()

	fmt.Println("compacting (removing orphans)...")

	// get all known hashes from DB
	// this might be slow if DB is huge.
	// for "simple", we load all keys.
	keys, err := store.ListKeys()
	if err != nil {
		return err
	}

	knownHashes := make(map[string]bool)
	for _, k := range keys {
		// if key is hash, add it.
		// if key is user key, we need to know the hash.
		// but we store full URL in DB: .../hash
		// so we can extract hash from URL.
		locs, _ := store.GetBlob(k)
		for _, loc := range locs {
			parts := strings.Split(loc, "/")
			if len(parts) > 0 {
				hash := parts[len(parts)-1]
				knownHashes[hash] = true
			}
		}
	}

	volumes := strings.Split(ctx.Volumes, ",")
	for _, vol := range volumes {
		vol = strings.TrimSpace(vol)
		fmt.Printf("scanning volume %s...\n", vol)

		resp, err := http.Get(vol + "/_list")
		if err != nil {
			fmt.Printf("failed to scan %s: %v\n", vol, err)
			continue
		}
		defer resp.Body.Close()

		var blobs []string
		if err := json.NewDecoder(resp.Body).Decode(&blobs); err != nil {
			continue
		}

		for _, hash := range blobs {
			if !knownHashes[hash] {
				fmt.Printf("deleting orphan %s on %s\n", hash, vol)
				// DELETE
				req, _ := http.NewRequest(http.MethodDelete, vol+"/"+hash, nil) // volume wrapper handles DELETE /hash
				http.DefaultClient.Do(req)
			}
		}
	}

	fmt.Println("compaction complete.")
	return nil
}
