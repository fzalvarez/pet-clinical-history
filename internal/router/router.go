package router

import (
	"net/http"

	mem "pet-clinical-history/internal/adapters/storage/memory"
	"pet-clinical-history/internal/domain/events"
	"pet-clinical-history/internal/domain/pets"
	"pet-clinical-history/internal/middleware"
	"pet-clinical-history/internal/ports/auth"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

type Options struct {
	AuthVerifier auth.AuthVerifier // puede ser nil (modo dev)
}

func NewRouter(opts Options) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)

	r.Use(middleware.AuthContext(opts.AuthVerifier))

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Repos in-memory
	petRepo := mem.NewPetRepo()
	eventRepo := mem.NewEventRepo()

	// Services por módulo
	petsSvc := pets.NewService(petRepo)
	eventsSvc := events.NewService(eventRepo)

	// Rutas por módulo
	pets.RegisterRoutes(r, petsSvc)
	events.RegisterRoutes(r, eventsSvc, petsSvc)

	return r
}
