package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"pet-clinical-history/internal/router"

	_ "pet-clinical-history/docs" // importa docs generados por swag
)

// @title Pet Clinical History API
// @version 1.0
// @description API REST para gestión de historiales clínicos de mascotas con sistema de delegación de acceso.
// @termsOfService http://swagger.io/terms/
//
// @contact.name API Support
// @contact.url https://odyssoft.com/support
// @contact.email support@odyssoft.com
//
// @license.name MIT
// @license.url https://opensource.org/licenses/MIT
//
// @host localhost:8080
// @BasePath /
//
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Token JWT obtenido de Odin-IAM. Formato: `Bearer <token>`
//
// @securityDefinitions.apikey DebugUserID
// @in header
// @name X-Debug-User-ID
// @description Solo en modo dev. Permite simular autenticación sin token real.

func main() {
	addr := ":8080"
	if v := os.Getenv("PORT"); v != "" {
		addr = ":" + v
	}

	// MVP: sin verifier para modo dev.
	// Más adelante aquí podrás wirear Odin-IAM (AuthVerifier real).
	r := router.NewRouter(router.Options{AuthVerifier: nil})

	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Arranca server en goroutine
	errCh := make(chan error, 1)
	go func() {
		log.Printf("starting server on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	// Espera señal o error fatal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Printf("shutdown signal received: %s", sig.String())
	case err := <-errCh:
		if err != nil {
			log.Fatalf("server error: %v", err)
		}
		// si errCh cierra sin error, caemos a shutdown igual
	}

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	} else {
		log.Printf("server stopped")
	}
}
