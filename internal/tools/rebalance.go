package tools

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

func Rebalance(ctx *Context) error {
	store, err := ctx.GetStore()
	if err != nil {
		return err
	}
	defer store.Close()

	ring := ctx.GetRing()

	fmt.Println("starting rebalance...")

	keys, err := store.ListKeys()
	if err != nil {
		return fmt.Errorf("failed to list keys: %v", err)
	}

	var movedCount int
	var errorCount int

	for _, key := range keys {
		// get current locations
		currentLocs, err := store.GetBlob(key)
		if err != nil {
			fmt.Printf("error getting blob %s: %v\n", key, err)
			errorCount++
			continue
		}

		// get desired locations from ring
		desiredNodes := ring.GetNodes(key, ctx.Replicas)
		desiredMap := make(map[string]bool)
		for _, node := range desiredNodes {
			desiredMap[node] = true
		}

		// check if we need to move
		// currentLocs are full URLs: http://vol:8081/ab/cd/hash
		// desiredNodes are base URLs: http://vol:8081

		// map current nodes
		currentNodes := make(map[string]string) // node -> fullURL
		for _, loc := range currentLocs {
			// extract base url
			// assume loc starts with http://host:port
			// simple hack: split by / and take first 3 parts?
			// http://localhost:8081/ab/cd/hash -> http://localhost:8081
			parts := strings.Split(loc, "/")
			if len(parts) < 3 {
				continue
			}
			base := strings.Join(parts[:3], "/")
			currentNodes[base] = loc
		}

		// Find missing nodes
		var missingNodes []string
		for node := range desiredMap {
			if _, ok := currentNodes[node]; !ok {
				missingNodes = append(missingNodes, node)
			}
		}

		if len(missingNodes) == 0 {
			continue
		}

		// we need to replicate to missing nodes
		// pick a source node
		if len(currentLocs) == 0 {
			fmt.Printf("blob %s has no locations! data lost?\n", key)
			errorCount++
			continue
		}
		sourceURL := currentLocs[0]

		// download blob
		resp, err := http.Get(sourceURL)
		if err != nil {
			fmt.Printf("failed to download %s from %s: %v\n", key, sourceURL, err)
			errorCount++
			continue
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			fmt.Printf("failed to read %s: %v\n", key, err)
			errorCount++
			continue
		}

		// upload to missing nodes
		var wg sync.WaitGroup
		var mu sync.Mutex
		var newLocs []string

		for _, targetNode := range missingNodes {
			wg.Add(1)
			go func(node string) {
				defer wg.Done()
				// PUT to node
				req, _ := http.NewRequest(http.MethodPut, node, bytes.NewReader(data))
				req.ContentLength = int64(len(data))

				resp, err := http.DefaultClient.Do(req)
				if err != nil || resp.StatusCode >= 300 {
					fmt.Printf("failed to replicate %s to %s\n", key, node)
					return
				}
				defer resp.Body.Close()

				hashBytes, _ := io.ReadAll(resp.Body)
				hash := string(hashBytes)

				u := fmt.Sprintf("%s/%s/%s/%s", node, hash[:2], hash[2:4], hash)

				mu.Lock()
				newLocs = append(newLocs, u)
				mu.Unlock()
			}(targetNode)
		}
		wg.Wait()

		if len(newLocs) > 0 {
			// update DB
			// append new locations
			allLocs := append(currentLocs, newLocs...)
			if err := store.PutBlob(key, allLocs); err != nil {
				fmt.Printf("failed to update DB for %s: %v\n", key, err)
			} else {
				movedCount++
				fmt.Printf("rebalanced %s (added %d replicas)\n", key, len(newLocs))
			}
		}
	}

	fmt.Printf("rebalance complete. moved: %d, errors: %d\n", movedCount, errorCount)
	return nil
}
