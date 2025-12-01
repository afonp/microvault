package tools

import (
	"fmt"
	"net/http"
)

func Verify(ctx *Context) error {
	store, err := ctx.GetStore()
	if err != nil {
		return err
	}
	defer store.Close()

	fmt.Println("verifying consistency...")

	keys, err := store.ListKeys()
	if err != nil {
		return err
	}

	var errors int
	for _, key := range keys {
		locs, err := store.GetBlob(key)
		if err != nil {
			fmt.Printf("error getting %s: %v\n", key, err)
			errors++
			continue
		}

		if len(locs) < ctx.Replicas {
			fmt.Printf("under-replicated: %s (%d/%d)\n", key, len(locs), ctx.Replicas)
			errors++
		}

		for _, loc := range locs {
			// check if file exists (HEAD request)
			resp, err := http.Head(loc)
			if err != nil {
				fmt.Printf("error checking %s at %s: %v\n", key, loc, err)
				errors++
				continue
			}
			if resp.StatusCode != http.StatusOK {
				fmt.Printf("missing: %s at %s (status %d)\n", key, loc, resp.StatusCode)
				errors++
			}
		}
	}

	if errors == 0 {
		fmt.Println("verification passed!")
	} else {
		fmt.Printf("verification failed with %d errors.\n", errors)
	}
	return nil
}
