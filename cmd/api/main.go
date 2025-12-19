package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"pet-clinical-history/internal/router"
)

func main() {
	addr := ":8080"
	if v := os.Getenv("PORT"); v != "" {
		addr = ":" + v
	}

	r := router.NewRouter(router.Options{AuthVerifier: nil}) // sin verifier para modo dev

	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Printf("starting server on %s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
