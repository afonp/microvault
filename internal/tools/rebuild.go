package tools

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func Rebuild(ctx *Context) error {
	store, err := ctx.GetStore()
	if err != nil {
		return err
	}
	defer store.Close()

	fmt.Println("rebuilding index...")

	volumes := strings.Split(ctx.Volumes, ",")
	for _, vol := range volumes {
		vol = strings.TrimSpace(vol)
		fmt.Printf("scanning volume %s...\n", vol)

		// fetch list of blobs from volume
		// we assume volume has a /_list endpoint that returns JSON list of hashes
		resp, err := http.Get(vol + "/_list")
		if err != nil {
			fmt.Printf("failed to scan volume %s: %v\n", vol, err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			fmt.Printf("volume %s returned status %d\n", vol, resp.StatusCode)
			continue
		}

		var blobs []string
		if err := json.NewDecoder(resp.Body).Decode(&blobs); err != nil {
			fmt.Printf("failed to decode response from %s: %v\n", vol, err)
			continue
		}

		for _, hash := range blobs {
			// if the db is gone, we only have the blob hashes from the files.
			// custom user keys can't be recovered since the volume only stores `/ab/cd/hash`.
			// a rebuild can only restore hash-based entries unless we start writing metadata,
			// which we currently don't.

			// during rebuild, register this volume as a location for the hash.
			// don't overwrite blindly: check if the entry exists and update or create it.

			currentLocs, _ := store.GetBlob(hash)

			// check if this volume is already in list
			exists := false
			targetURL := fmt.Sprintf("%s/%s/%s/%s", vol, hash[:2], hash[2:4], hash)

			for _, loc := range currentLocs {
				if loc == targetURL {
					exists = true
					break
				}
			}

			if !exists {
				currentLocs = append(currentLocs, targetURL)
				if err := store.PutBlob(hash, currentLocs); err != nil {
					fmt.Printf("failed to update index for %s: %v\n", hash, err)
				}
			}
		}
	}

	fmt.Println("rebuild complete.")
	return nil
}
