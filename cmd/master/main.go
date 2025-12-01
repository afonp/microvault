package main

import (
	"flag"
	"log"
	"net/http"

	"strings"

	"github.com/afonp/microvault/internal/api"
	"github.com/afonp/microvault/internal/db"
	"github.com/afonp/microvault/internal/hashing"
)

func main() {
	port := flag.String("port", "8080", "port to listen on")
	dbPath := flag.String("db", "metadata.db", "path to metadata database")
	volumes := flag.String("volumes", "http://localhost:8081", "comma-separated list of volume servers")
	replicas := flag.Int("replicas", 3, "number of replicas")
	flag.Parse()

	store, err := db.NewStore(*dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer store.Close()

	ring := hashing.NewRing(*replicas) // replicas for virtual nodes
	for _, v := range strings.Split(*volumes, ",") {
		ring.AddNode(strings.TrimSpace(v))
	}

	handler := api.NewHandler(store, ring, *replicas)

	http.HandleFunc("/blob/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead:
			handler.ServeBlob(w, r)
		case http.MethodPut:
			handler.PutBlob(w, r)
		case http.MethodDelete:
			handler.DeleteBlob(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	log.Printf("master server listening on :%s", *port)
	if err := http.ListenAndServe(":"+*port, nil); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
