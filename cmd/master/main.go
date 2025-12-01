package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/afonp/microvault/internal/api"
	"github.com/afonp/microvault/internal/db"
)

func main() {
	port := flag.String("port", "8080", "port to listen on")
	dbPath := flag.String("db", "metadata.db", "path to metadata database")
	volumeURL := flag.String("volume", "http://localhost:8081", "url of the volume server")
	flag.Parse()

	store, err := db.NewStore(*dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer store.Close()

	handler := api.NewHandler(store, *volumeURL)

	http.HandleFunc("/blob/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
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
