package router

import (
	"database/sql"
	"net/http"
	"os"

	mem "pet-clinical-history/internal/adapters/storage/memory"
	pg "pet-clinical-history/internal/adapters/storage/postgres"
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

	// Opcional: si viene, usa Postgres. Si no, in-memory.
	DB *sql.DB
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

	var (
		petRepo    pets.Repository
		eventRepo  events.Repository
		grantsRepo accessgrants.Repository
	)

	// Repos in-memory
	/* petRepo := mem.NewPetRepo()
	eventRepo := mem.NewEventRepo()
	grantsRepo := mem.NewAccessGrantsRepo() */

	// Si no te pasan DB explícita, intenta por env (para dev/handoff)
	db := opts.DB
	if db == nil {
		if dsn := os.Getenv("DB_DSN"); dsn != "" {
			opened, err := pg.Open(dsn)
			if err == nil {
				db = opened
			}
		}
	}

	if db != nil {
		petRepo = pg.NewPetsRepo(db)
		eventRepo = pg.NewEventsRepo(db)
		grantsRepo = pg.NewAccessGrantsRepo(db)
	} else {
		petRepo = mem.NewPetRepo()
		eventRepo = mem.NewEventRepo()
		grantsRepo = mem.NewAccessGrantsRepo()
	}

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
