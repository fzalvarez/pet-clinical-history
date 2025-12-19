package events

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"pet-clinical-history/internal/domain/pets"
	"pet-clinical-history/internal/middleware"

	"github.com/go-chi/chi/v5"
)

func RegisterRoutes(r chi.Router, svc *Service, petsSvc *pets.Service) {
	r.Route("/pets/{petID}/events", func(er chi.Router) {
		er.Post("/", createEventHandler(svc, petsSvc))
		er.Get("/", listEventsHandler(svc, petsSvc))
	})
}

type createEventRequest struct {
	Type       EventType  `json:"type"`
	OccurredAt string     `json:"occurred_at"` // RFC3339
	Title      string     `json:"title"`
	Notes      string     `json:"notes"`
	Source     Source     `json:"source"`     // opcional
	Visibility Visibility `json:"visibility"` // opcional
}

type eventResponse struct {
	ID         string      `json:"id"`
	PetID      string      `json:"pet_id"`
	Type       EventType   `json:"type"`
	OccurredAt time.Time   `json:"occurred_at"`
	RecordedAt time.Time   `json:"recorded_at"`
	Title      string      `json:"title"`
	Notes      string      `json:"notes"`
	ActorType  ActorType   `json:"actor_type"`
	ActorID    string      `json:"actor_id"`
	Source     Source      `json:"source"`
	Visibility Visibility  `json:"visibility"`
	Status     EventStatus `json:"status"`
}

func createEventHandler(svc *Service, petsSvc *pets.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.GetClaims(r.Context())
		if !ok || strings.TrimSpace(claims.UserID) == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		petID := chi.URLParam(r, "petID")
		p, err := petsSvc.GetByID(r.Context(), petID)
		if err != nil {
			http.Error(w, "pet not found", http.StatusNotFound)
			return
		}
		if p.OwnerUserID != claims.UserID {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		var req createEventRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		t, err := time.Parse(time.RFC3339, req.OccurredAt)
		if err != nil {
			http.Error(w, "occurred_at must be RFC3339", http.StatusBadRequest)
			return
		}

		e, err := svc.Create(r.Context(), petID, Actor{
			Type: ActorTypeOwnerUser,
			ID:   claims.UserID,
		}, CreateInput{
			Type:       req.Type,
			OccurredAt: t,
			Title:      req.Title,
			Notes:      req.Notes,
			Source:     req.Source,
			Visibility: req.Visibility,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		writeJSON(w, http.StatusCreated, toEventResponse(e))
	}
}

func listEventsHandler(svc *Service, petsSvc *pets.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.GetClaims(r.Context())
		if !ok || strings.TrimSpace(claims.UserID) == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		petID := chi.URLParam(r, "petID")
		p, err := petsSvc.GetByID(r.Context(), petID)
		if err != nil {
			http.Error(w, "pet not found", http.StatusNotFound)
			return
		}
		if p.OwnerUserID != claims.UserID {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		limit := 50
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
				limit = n
			}
		}

		filter := ListFilter{Limit: limit}
		items, err := svc.ListByPet(r.Context(), petID, filter)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		out := make([]eventResponse, 0, len(items))
		for _, e := range items {
			out = append(out, toEventResponse(e))
		}

		writeJSON(w, http.StatusOK, out)
	}
}

func toEventResponse(e PetEvent) eventResponse {
	return eventResponse{
		ID:         e.ID,
		PetID:      e.PetID,
		Type:       e.Type,
		OccurredAt: e.OccurredAt,
		RecordedAt: e.RecordedAt,
		Title:      e.Title,
		Notes:      e.Notes,
		ActorType:  e.Actor.Type,
		ActorID:    e.Actor.ID,
		Source:     e.Source,
		Visibility: e.Visibility,
		Status:     e.Status,
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
