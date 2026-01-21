package events

import (
	"encoding/json"
	"errors"
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

		// Anular (void) evento (owner o delegado con events:void)
		er.Post("/{eventID}/void", voidEventHandler(svc, petsSvc, grantsSvc))
	})
}

// createEventRequest es el cuerpo de la solicitud para registrar un nuevo evento clínico.
type createEventRequest struct {
	Type       EventType  `json:"type" enums:"NOTE,MEDICAL_VISIT,VACCINE,DEWORMING,BATH,PROFILE_UPDATED,WEIGHT_RECORDED,MEDICATION_PRESCRIBED,FLEA_TREATMENT,ATTACHMENT_ADDED"`
	OccurredAt string     `json:"occurred_at"` // RFC3339
	Title      string     `json:"title"`
	Notes      string     `json:"notes"`
	Source     Source     `json:"source"`     // opcional
	Visibility Visibility `json:"visibility"` // opcional
}

// eventResponse representa un evento clínico de la mascota devuelto por la API.
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

// createEventHandler godoc
// @Summary Crear evento de mascota
// @Description Crea un nuevo evento clínico para la mascota indicada. El dueño siempre puede crear eventos. Un delegado necesita un grant activo con scope `events:create`. Autenticación: `X-Debug-User-ID` (dev) o `Authorization: Bearer <token>` (prod).
// @Tags events
// @Accept json
// @Produce json
// @Param X-Debug-User-ID header string false "Solo en modo dev, ID de usuario para depuración"
// @Param Authorization header string false "Bearer token en producción"
// @Param petID path string true "ID de la mascota"
// @Param payload body createEventRequest true "Datos del evento; occurred_at en formato RFC3339"
// @Success 201 {object} eventResponse
// @Failure 400 {string} string "invalid json / occurred_at inválido / reglas de negocio"
// @Failure 401 {string} string "unauthorized"
// @Failure 403 {string} string "forbidden"
// @Failure 404 {string} string "pet not found"
// @Router /pets/{petID}/events [post]
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

// listEventsHandler godoc
// @Summary Listar eventos de una mascota
// @Description Lista los eventos clínicos de una mascota. El dueño siempre puede verlos. Un delegado necesita un grant activo con scope `events:read`. Autenticación: `X-Debug-User-ID` (dev) o `Authorization: Bearer <token>` (prod). Permite filtrar por tipos, rango de fechas y texto.
// @Tags events
// @Accept json
// @Produce json
// @Param X-Debug-User-ID header string false "Solo en modo dev, ID de usuario para depuración"
// @Param Authorization header string false "Bearer token en producción"
// @Param petID path string true "ID de la mascota"
// @Param limit query int false "Máximo de eventos a devolver (1-200). Por defecto 50"
// @Param types query string false "Lista CSV de tipos de evento a incluir (ej: MEDICAL_VISIT,BATH)"
// @Param from query string false "Fecha/hora mínima occurred_at (RFC3339)"
// @Param to query string false "Fecha/hora máxima occurred_at (RFC3339)"
// @Param q query string false "Texto de búsqueda libre en título/notas"
// @Success 200 {array} eventResponse
// @Failure 400 {string} string "Parámetros de filtro inválidos"
// @Failure 401 {string} string "unauthorized"
// @Failure 403 {string} string "forbidden"
// @Failure 404 {string} string "pet not found"
// @Failure 500 {string} string "internal error"
// @Router /pets/{petID}/events [get]
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

// voidEventHandler godoc
// @Summary Anular (void) un evento
// @Description Anula un evento existente de la mascota. El dueño siempre puede anular. Un delegado necesita un grant activo con scope `events:void`. Autenticación: `X-Debug-User-ID` (dev) o `Authorization: Bearer <token>` (prod).
// @Tags events
// @Accept json
// @Produce json
// @Param X-Debug-User-ID header string false "Solo en modo dev, ID de usuario para depuración"
// @Param Authorization header string false "Bearer token en producción"
// @Param petID path string true "ID de la mascota"
// @Param eventID path string true "ID del evento"
// @Success 200 {object} eventResponse
// @Failure 401 {string} string "unauthorized"
// @Failure 403 {string} string "forbidden"
// @Failure 404 {string} string "event not found"
// @Failure 500 {string} string "internal error"
// @Router /pets/{petID}/events/{eventID}/void [post]
func voidEventHandler(svc *Service, petsSvc *pets.Service, grantsSvc *accessgrants.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.GetClaims(r.Context())
		if !ok || strings.TrimSpace(claims.UserID) == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		petID := chi.URLParam(r, "petID")
		eventID := chi.URLParam(r, "eventID")

		// Pet existe
		p, err := petsSvc.GetByID(r.Context(), petID)
		if err != nil {
			http.Error(w, "pet not found", http.StatusNotFound)
			return
		}

		// Permisos (primero, para no filtrar si existe el evento)
		// - Owner: siempre permitido
		// - Delegado: requiere grant activo con ScopeEventsVoid
		if p.OwnerUserID != claims.UserID {
			g, err := grantsSvc.GetActiveGrant(r.Context(), petID, claims.UserID)
			if err != nil || !accessgrants.HasScope(g, accessgrants.ScopeEventsVoid) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
		}

		// Evento existe y pertenece al pet
		ev, err := svc.GetByID(r.Context(), eventID)
		if err != nil || strings.TrimSpace(ev.ID) == "" || ev.PetID != petID {
			http.Error(w, "event not found", http.StatusNotFound)
			return
		}

		updated, err := svc.Void(r.Context(), eventID)
		if err != nil {
			// MVP: tratamos "not found" como 404 (evita 500 innecesarios en memoria)
			if strings.Contains(strings.ToLower(err.Error()), "not found") {
				http.Error(w, "event not found", http.StatusNotFound)
				return
			}
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, toEventResponse(updated))
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
			return ListFilter{}, errors.New("from must be RFC3339")
		}
		filter.From = &t
	}
	if v := strings.TrimSpace(r.URL.Query().Get("to")); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return ListFilter{}, errors.New("to must be RFC3339")
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
