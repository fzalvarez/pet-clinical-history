package pets

import (
	"encoding/json"
	"net/http"
	"time"

	"pet-clinical-history/internal/middleware"

	"github.com/go-chi/chi/v5"
)

func RegisterRoutes(r chi.Router, svc *Service) {
	r.Route("/pets", func(pr chi.Router) {
		pr.Post("/", createPetHandler(svc))
		pr.Get("/", listPetsHandler(svc))
	})
}

type createPetRequest struct {
	Name      string `json:"name"`
	Species   string `json:"species"`
	Breed     string `json:"breed"`
	Sex       string `json:"sex"`
	BirthDate string `json:"birth_date"` // YYYY-MM-DD opcional
	Notes     string `json:"notes"`
}

type petResponse struct {
	ID          string     `json:"id"`
	OwnerUserID string     `json:"owner_user_id"`
	Name        string     `json:"name"`
	Species     string     `json:"species"`
	Breed       string     `json:"breed"`
	Sex         string     `json:"sex"`
	BirthDate   *time.Time `json:"birth_date,omitempty"`
	Notes       string     `json:"notes"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func createPetHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.GetClaims(r.Context())
		if !ok || claims.UserID == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var req createPetRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		var bd *time.Time
		if req.BirthDate != "" {
			t, err := time.Parse("2006-01-02", req.BirthDate)
			if err != nil {
				http.Error(w, "birth_date must be YYYY-MM-DD", http.StatusBadRequest)
				return
			}
			bd = &t
		}

		p, err := svc.Create(r.Context(), claims.UserID, CreateInput{
			Name:      req.Name,
			Species:   req.Species,
			Breed:     req.Breed,
			Sex:       req.Sex,
			BirthDate: bd,
			Notes:     req.Notes,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		writeJSON(w, http.StatusCreated, toPetResponse(p))
	}
}

func listPetsHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.GetClaims(r.Context())
		if !ok || claims.UserID == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		items, err := svc.ListByOwner(r.Context(), claims.UserID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		out := make([]petResponse, 0, len(items))
		for _, p := range items {
			out = append(out, toPetResponse(p))
		}

		writeJSON(w, http.StatusOK, out)
	}
}

func toPetResponse(p Pet) petResponse {
	return petResponse{
		ID:          p.ID,
		OwnerUserID: p.OwnerUserID,
		Name:        p.Name,
		Species:     p.Species,
		Breed:       p.Breed,
		Sex:         p.Sex,
		BirthDate:   p.BirthDate,
		Notes:       p.Notes,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

// writeJSON está duplicado intencionalmente en handlers de distintos módulos (pets/events)
// para evitar crear paquetes/helpers compartidos demasiado pronto.
// Si más adelante se repite en más módulos, recién conviene extraerlo a un helper común.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
