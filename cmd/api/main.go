package main

import (
	"log"
	"net/http"

	"github.com/kev-chen369/shlms/internal/httpapi"
)

func main() {
	server := &http.Server{
		Addr:    ":8080",
		Handler: httpapi.NewRouter(),
	}
	log.Printf("api listening on %s", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
