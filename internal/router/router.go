package router

import (
	"net/http"

	mem "pet-clinical-history/internal/adapters/storage/memory"
	"pet-clinical-history/internal/domain/accessgrants"
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
	grantsRepo := mem.NewAccessGrantsRepo()

	// Services por módulo
	petsSvc := pets.NewService(petRepo)
	eventsSvc := events.NewService(eventRepo)
	grantsSvc := accessgrants.NewService(grantsRepo)

	// Rutas por módulo
	pets.RegisterRoutes(r, petsSvc, grantsSvc)

	//events.RegisterRoutes(r, eventsSvc, petsSvc) // en el siguiente paso, lo haremos validar delegados
	events.RegisterRoutes(r, eventsSvc, petsSvc, grantsSvc)
	accessgrants.RegisterRoutes(r, grantsSvc, petsSvc)

	return r
}
