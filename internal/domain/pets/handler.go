package pets

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"pet-clinical-history/internal/domain/accessgrants"
	"pet-clinical-history/internal/middleware"

	"github.com/go-chi/chi/v5"
)

func RegisterRoutes(r chi.Router, svc *Service, grantsSvc *accessgrants.Service) {
	// Pets (owner)
	r.Route("/pets", func(pr chi.Router) {
		pr.Post("/", createPetHandler(svc))
		pr.Get("/", listPetsHandler(svc))

		// Perfil de mascota (owner o delegado con pet:read)
		pr.Get("/{petID}", getPetHandler(svc, grantsSvc))

		// Actualizar mascota (owner o delegado con pet:write)
		pr.Patch("/{petID}", updatePetHandler(svc, grantsSvc))
	})

	// Mascotas compartidas conmigo (delegado)
	r.Get("/me/pets", listMySharedPetsHandler(svc, grantsSvc))
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

type updatePetRequest struct {
	// Punteros para PATCH real: nil = no tocar.
	Name      *string `json:"name"`
	Species   *string `json:"species"`
	Breed     *string `json:"breed"`
	Sex       *string `json:"sex"`
	BirthDate *string `json:"birth_date"` // YYYY-MM-DD. Para limpiar: enviar null (ver nota abajo)
	Notes     *string `json:"notes"`
}

// Para permitir "birth_date": null y diferenciarlo de "no enviado",
// usamos este wrapper que detecta presencia del campo.
type patchBirthDate struct {
	Present bool
	Value   *string
}

type sharedPetResponse struct {
	Pet    petResponse          `json:"pet"`
	Grant  sharedGrantSummary   `json:"grant"`
	Scopes []accessgrants.Scope `json:"scopes"` // redundante pero útil para UI
}

type sharedGrantSummary struct {
	ID     string              `json:"id"`
	Status accessgrants.Status `json:"status"`
}

func createPetHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.GetClaims(r.Context())
		if !ok || strings.TrimSpace(claims.UserID) == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var req createPetRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		var bd *time.Time
		if strings.TrimSpace(req.BirthDate) != "" {
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
	// Owner-only (sin mezclar shared)
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.GetClaims(r.Context())
		if !ok || strings.TrimSpace(claims.UserID) == "" {
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

func getPetHandler(svc *Service, grantsSvc *accessgrants.Service) http.HandlerFunc {
	// Owner bypass, delegado requiere pet:read
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.GetClaims(r.Context())
		if !ok || strings.TrimSpace(claims.UserID) == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		petID := chi.URLParam(r, "petID")
		p, err := svc.GetByID(r.Context(), petID)
		if err != nil {
			http.Error(w, "pet not found", http.StatusNotFound)
			return
		}

		// Owner bypass
		if p.OwnerUserID != claims.UserID {
			g, err := grantsSvc.GetActiveGrant(r.Context(), petID, claims.UserID)
			if err != nil || !accessgrants.HasScope(g, accessgrants.ScopePetRead) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
		}

		writeJSON(w, http.StatusOK, toPetResponse(p))
	}
}

// updatePetHandler aplica permisos:
// - owner bypass
// - delegado requiere grant activo + scope pet:edit_profile
func updatePetHandler(svc *Service, grantsSvc *accessgrants.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.GetClaims(r.Context())
		if !ok || strings.TrimSpace(claims.UserID) == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		petID := chi.URLParam(r, "petID")
		current, err := svc.GetByID(r.Context(), petID)
		if err != nil {
			http.Error(w, "pet not found", http.StatusNotFound)
			return
		}

		// Authorize: owner bypass, else grant + scope
		if current.OwnerUserID != claims.UserID {
			g, err := grantsSvc.GetActiveGrant(r.Context(), petID, claims.UserID)
			if err != nil || !accessgrants.HasScope(g, accessgrants.ScopePetEditProfile) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
		}

		// Decode PATCH body
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()

		// Para soportar birth_date: null, necesitamos detectar presencia del campo.
		// Estrategia: decodificar a map primero para ver si "birth_date" estuvo presente.
		var raw map[string]json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		// Decode campos simples usando un struct auxiliar sobre raw (sin birth_date)
		var req updatePetRequest
		{
			// Re-marshal a JSON y decode al struct para reutilizar tags
			// (simple y suficientemente eficiente para MVP)
			b, _ := json.Marshal(raw)
			if err := json.Unmarshal(b, &req); err != nil {
				http.Error(w, "invalid json", http.StatusBadRequest)
				return
			}
		}

		// Detectar presencia de birth_date (para permitir null = limpiar)
		bd := patchBirthDate{Present: false, Value: nil}
		if v, exists := raw["birth_date"]; exists {
			bd.Present = true
			if string(v) == "null" {
				bd.Value = nil
			} else {
				var s string
				if err := json.Unmarshal(v, &s); err != nil {
					http.Error(w, "birth_date must be YYYY-MM-DD or null", http.StatusBadRequest)
					return
				}
				bd.Value = &s
			}
		}

		// Call use-case
		updated, err := svc.UpdateProfile(r.Context(), petID, claims.UserID, UpdateProfileInput{
			Name:      req.Name,
			Species:   req.Species,
			Breed:     req.Breed,
			Sex:       req.Sex,
			BirthDate: bd, // wrapper con "present"
			Notes:     req.Notes,
		})
		if err != nil {
			switch err {
			case ErrInvalidInput:
				http.Error(w, err.Error(), http.StatusBadRequest)
			case ErrNotFound:
				http.Error(w, "pet not found", http.StatusNotFound)
			default:
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
			return
		}

		writeJSON(w, http.StatusOK, toPetResponse(updated))
	}
}

func listMySharedPetsHandler(svc *Service, grantsSvc *accessgrants.Service) http.HandlerFunc {
	// Devuelve mascotas compartidas conmigo (grants active con pet:read)
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.GetClaims(r.Context())
		if !ok || strings.TrimSpace(claims.UserID) == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		grants, err := grantsSvc.ListByGrantee(r.Context(), claims.UserID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		seen := map[string]struct{}{}
		out := make([]sharedPetResponse, 0)

		for _, g := range grants {
			if g.Status != accessgrants.StatusActive {
				continue
			}
			// Para mostrar perfil, exigimos pet:read.
			if !accessgrants.HasScope(g, accessgrants.ScopePetRead) {
				continue
			}
			if _, ok := seen[g.PetID]; ok {
				continue
			}
			seen[g.PetID] = struct{}{}

			p, err := svc.GetByID(r.Context(), g.PetID)
			if err != nil {
				// tolera grants huérfanos en MVP in-memory
				continue
			}

			out = append(out, sharedPetResponse{
				Pet: toPetResponse(p),
				Grant: sharedGrantSummary{
					ID:     g.ID,
					Status: g.Status,
				},
				Scopes: g.Scopes,
			})
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
