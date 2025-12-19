package events

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"pet-clinical-history/internal/domain/accessgrants"
	"pet-clinical-history/internal/domain/pets"
	"pet-clinical-history/internal/middleware"

	"github.com/go-chi/chi/v5"
)

func RegisterRoutes(r chi.Router, svc *Service, petsSvc *pets.Service, grantsSvc *accessgrants.Service) {
	r.Route("/pets/{petID}/events", func(er chi.Router) {
		er.Post("/", createEventHandler(svc, petsSvc, grantsSvc))
		er.Get("/", listEventsHandler(svc, petsSvc, grantsSvc))
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

func createEventHandler(svc *Service, petsSvc *pets.Service, grantsSvc *accessgrants.Service) http.HandlerFunc {
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

		actorType := ActorTypeOwnerUser

		// Permisos:
		// - Owner: siempre permitido
		// - Delegado: requiere grant activo con ScopeEventsCreate
		if p.OwnerUserID != claims.UserID {
			g, err := grantsSvc.GetActiveGrant(r.Context(), petID, claims.UserID)
			if err != nil || !accessgrants.HasScope(g, accessgrants.ScopeEventsCreate) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			actorType = ActorTypeDelegateUser
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
			Type: actorType,
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

func listEventsHandler(svc *Service, petsSvc *pets.Service, grantsSvc *accessgrants.Service) http.HandlerFunc {
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

		// Permisos:
		// - Owner: siempre permitido
		// - Delegado: requiere grant activo con ScopeEventsRead
		if p.OwnerUserID != claims.UserID {
			g, err := grantsSvc.GetActiveGrant(r.Context(), petID, claims.UserID)
			if err != nil || !accessgrants.HasScope(g, accessgrants.ScopeEventsRead) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
		}

		filter, err := parseListFilter(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

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

func parseListFilter(r *http.Request) (ListFilter, error) {
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}

	filter := ListFilter{Limit: limit}

	// types=MEDICAL_VISIT,BATH
	if v := strings.TrimSpace(r.URL.Query().Get("types")); v != "" {
		parts := strings.Split(v, ",")
		out := make([]EventType, 0, len(parts))
		for _, p := range parts {
			t := EventType(strings.TrimSpace(p))
			if t == "" {
				continue
			}
			out = append(out, t)
		}
		if len(out) > 0 {
			filter.Types = out
		}
	}

	// from/to RFC3339
	if v := strings.TrimSpace(r.URL.Query().Get("from")); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return ListFilter{}, ErrInvalidInput
		}
		filter.From = &t
	}
	if v := strings.TrimSpace(r.URL.Query().Get("to")); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return ListFilter{}, ErrInvalidInput
		}
		filter.To = &t
	}

	// q
	if v := strings.TrimSpace(r.URL.Query().Get("q")); v != "" {
		filter.Query = v
	}

	return filter, nil
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

// writeJSON está duplicado intencionalmente en handlers de distintos módulos
// para evitar crear paquetes/helpers compartidos demasiado pronto.
// Si más adelante se repite en más módulos, recién conviene extraerlo a un helper común.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
