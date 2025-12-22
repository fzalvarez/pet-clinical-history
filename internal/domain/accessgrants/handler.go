package accessgrants

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"pet-clinical-history/internal/middleware"

	"github.com/go-chi/chi/v5"
)

// PetOwnerLookup evita importar el paquete pets (rompe ciclos).
type PetOwnerLookup interface {
	OwnerOf(ctx context.Context, petID string) (string, error)
}

func RegisterRoutes(r chi.Router, svc *Service, petOwners PetOwnerLookup) {
	// Owner actions scoped by pet
	r.Route("/pets/{petID}/grants", func(gr chi.Router) {
		gr.Post("/", inviteGrantHandler(svc, petOwners))
		gr.Get("/", listGrantsByPetHandler(svc, petOwners))
	})

	// Grantee/Owner actions scoped by grant id
	r.Route("/grants/{grantID}", func(gr chi.Router) {
		gr.Post("/accept", acceptGrantHandler(svc))
		gr.Post("/revoke", revokeGrantHandler(svc))
	})

	// Delegado: ver sus invitaciones / grants
	r.Route("/me/grants", func(mr chi.Router) {
		mr.Get("/", listMyGrantsHandler(svc))
	})
}

// inviteGrantRequest es el cuerpo de la solicitud para invitar a un delegado a una mascota.
type inviteGrantRequest struct {
	GranteeUserID string  `json:"grantee_user_id"`
	Scopes        []Scope `json:"scopes"`
}

// grantResponse representa un grant de acceso delegado en las respuestas de la API.
type grantResponse struct {
	ID            string     `json:"id"`
	PetID         string     `json:"pet_id"`
	OwnerUserID   string     `json:"owner_user_id"`
	GranteeUserID string     `json:"grantee_user_id"`
	Scopes        []Scope    `json:"scopes"`
	Status        Status     `json:"status"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	RevokedAt     *time.Time `json:"revoked_at,omitempty"`
}

// inviteGrantHandler godoc
// @Summary Invitar delegado a una mascota
// @Description Crea una invitación (grant) para que otro usuario acceda a la mascota. Solo el owner de la mascota puede invitar. Autenticación: `X-Debug-User-ID` (dev) o `Authorization: Bearer <token>` (prod).
// @Tags accessgrants
// @Accept json
// @Produce json
// @Param X-Debug-User-ID header string false "Solo en modo dev, ID de usuario para depuración"
// @Param Authorization header string false "Bearer token en producción"
// @Param petID path string true "ID de la mascota compartida"
// @Param payload body inviteGrantRequest true "Datos de la invitación (usuario delegado y scopes otorgados)"
// @Success 201 {object} grantResponse
// @Failure 400 {string} string "invalid json / invalid input / grantee_user_id requerido"
// @Failure 401 {string} string "unauthorized"
// @Failure 403 {string} string "forbidden"
// @Failure 404 {string} string "pet not found"
// @Failure 500 {string} string "internal error"
// @Router /pets/{petID}/grants [post]
func inviteGrantHandler(svc *Service, petOwners PetOwnerLookup) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.GetClaims(r.Context())
		if !ok || strings.TrimSpace(claims.UserID) == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		petID := chi.URLParam(r, "petID")

		ownerID, err := petOwners.OwnerOf(r.Context(), petID)
		if err != nil || strings.TrimSpace(ownerID) == "" {
			http.Error(w, "pet not found", http.StatusNotFound)
			return
		}
		if ownerID != claims.UserID {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		var req inviteGrantRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.GranteeUserID) == "" {
			http.Error(w, "grantee_user_id required", http.StatusBadRequest)
			return
		}

		g, err := svc.Invite(r.Context(), InviteInput{
			PetID:         petID,
			OwnerUserID:   claims.UserID,
			GranteeUserID: strings.TrimSpace(req.GranteeUserID),
			Scopes:        req.Scopes,
		})
		if err != nil {
			switch err {
			case ErrInvalidInput:
				http.Error(w, err.Error(), http.StatusBadRequest)
			default:
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
			return
		}

		writeJSON(w, http.StatusCreated, toGrantResponse(g))
	}
}

// listGrantsByPetHandler godoc
// @Summary Listar grants por mascota
// @Description Lista todos los grants asociados a una mascota. Solo el owner de la mascota puede verlos. Autenticación: `X-Debug-User-ID` (dev) o `Authorization: Bearer <token>` (prod).
// @Tags accessgrants
// @Accept json
// @Produce json
// @Param X-Debug-User-ID header string false "Solo en modo dev, ID de usuario para depuración"
// @Param Authorization header string false "Bearer token en producción"
// @Param petID path string true "ID de la mascota"
// @Success 200 {array} grantResponse
// @Failure 401 {string} string "unauthorized"
// @Failure 403 {string} string "forbidden"
// @Failure 404 {string} string "pet not found"
// @Failure 500 {string} string "internal error"
// @Router /pets/{petID}/grants [get]
func listGrantsByPetHandler(svc *Service, petOwners PetOwnerLookup) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.GetClaims(r.Context())
		if !ok || strings.TrimSpace(claims.UserID) == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		petID := chi.URLParam(r, "petID")

		ownerID, err := petOwners.OwnerOf(r.Context(), petID)
		if err != nil || strings.TrimSpace(ownerID) == "" {
			http.Error(w, "pet not found", http.StatusNotFound)
			return
		}
		if ownerID != claims.UserID {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		items, err := svc.ListByPet(r.Context(), petID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		out := make([]grantResponse, 0, len(items))
		for _, g := range items {
			out = append(out, toGrantResponse(g))
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// listMyGrantsHandler godoc
// @Summary Listar mis grants como delegado
// @Description Lista los grants donde el usuario autenticado es el delegado (grantee). Opcionalmente filtra por estado mediante `status=invited,active`. Autenticación: `X-Debug-User-ID` (dev) o `Authorization: Bearer <token>` (prod).
// @Tags accessgrants
// @Accept json
// @Produce json
// @Param X-Debug-User-ID header string false "Solo en modo dev, ID de usuario para depuración"
// @Param Authorization header string false "Bearer token en producción"
// @Param status query string false "Lista CSV de estados permitidos (ej: invited,active)"
// @Success 200 {array} grantResponse
// @Failure 401 {string} string "unauthorized"
// @Failure 500 {string} string "internal error"
// @Router /me/grants [get]
func listMyGrantsHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.GetClaims(r.Context())
		if !ok || strings.TrimSpace(claims.UserID) == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// status=invited,active (CSV opcional)
		allowed := parseStatusFilter(r.URL.Query().Get("status"))

		items, err := svc.ListByGrantee(r.Context(), claims.UserID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		// Filtrar por status si se especificó
		if len(allowed) > 0 {
			filtered := make([]Grant, 0, len(items))
			for _, g := range items {
				if _, ok := allowed[g.Status]; ok {
					filtered = append(filtered, g)
				}
			}
			items = filtered
		}

		out := make([]grantResponse, 0, len(items))
		for _, g := range items {
			out = append(out, toGrantResponse(g))
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// acceptGrantHandler godoc
// @Summary Aceptar una invitación de grant
// @Description Acepta una invitación pendiente para que el usuario autenticado se convierta en delegado de una mascota. Solo el grantee puede aceptar su invitación. Autenticación: `X-Debug-User-ID` (dev) o `Authorization: Bearer <token>` (prod).
// @Tags accessgrants
// @Accept json
// @Produce json
// @Param X-Debug-User-ID header string false "Solo en modo dev, ID de usuario para depuración"
// @Param Authorization header string false "Bearer token en producción"
// @Param grantID path string true "ID del grant a aceptar"
// @Success 200 {object} grantResponse
// @Failure 400 {string} string "invalid input"
// @Failure 401 {string} string "unauthorized"
// @Failure 403 {string} string "forbidden"
// @Failure 404 {string} string "not found"
// @Failure 409 {string} string "bad state para aceptar (ej: ya aceptado/revocado)"
// @Failure 500 {string} string "internal error"
// @Router /grants/{grantID}/accept [post]
func acceptGrantHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.GetClaims(r.Context())
		if !ok || strings.TrimSpace(claims.UserID) == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		grantID := chi.URLParam(r, "grantID")
		g, err := svc.Accept(r.Context(), grantID, claims.UserID)
		if err != nil {
			switch err {
			case ErrInvalidInput:
				http.Error(w, err.Error(), http.StatusBadRequest)
			case ErrForbidden:
				http.Error(w, "forbidden", http.StatusForbidden)
			case ErrNotFound:
				http.Error(w, "not found", http.StatusNotFound)
			case ErrBadState:
				http.Error(w, err.Error(), http.StatusConflict)
			default:
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
			return
		}

		writeJSON(w, http.StatusOK, toGrantResponse(g))
	}
}

// revokeGrantHandler godoc
// @Summary Revocar un grant
// @Description Revoca un grant existente. Puede ser ejecutado por el owner o, según la lógica de negocio, por el delegado cuando corresponda. Autenticación: `X-Debug-User-ID` (dev) o `Authorization: Bearer <token>` (prod).
// @Tags accessgrants
// @Accept json
// @Produce json
// @Param X-Debug-User-ID header string false "Solo en modo dev, ID de usuario para depuración"
// @Param Authorization header string false "Bearer token en producción"
// @Param grantID path string true "ID del grant a revocar"
// @Success 200 {object} grantResponse
// @Failure 400 {string} string "invalid input"
// @Failure 401 {string} string "unauthorized"
// @Failure 403 {string} string "forbidden"
// @Failure 404 {string} string "not found"
// @Failure 500 {string} string "internal error"
// @Router /grants/{grantID}/revoke [post]
func revokeGrantHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.GetClaims(r.Context())
		if !ok || strings.TrimSpace(claims.UserID) == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		grantID := chi.URLParam(r, "grantID")
		g, err := svc.Revoke(r.Context(), grantID, claims.UserID)
		if err != nil {
			switch err {
			case ErrInvalidInput:
				http.Error(w, err.Error(), http.StatusBadRequest)
			case ErrForbidden:
				http.Error(w, "forbidden", http.StatusForbidden)
			case ErrNotFound:
				http.Error(w, "not found", http.StatusNotFound)
			default:
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
			return
		}

		writeJSON(w, http.StatusOK, toGrantResponse(g))
	}
}

func toGrantResponse(g Grant) grantResponse {
	return grantResponse{
		ID:            g.ID,
		PetID:         g.PetID,
		OwnerUserID:   g.OwnerUserID,
		GranteeUserID: g.GranteeUserID,
		Scopes:        g.Scopes,
		Status:        g.Status,
		CreatedAt:     g.CreatedAt,
		UpdatedAt:     g.UpdatedAt,
		RevokedAt:     g.RevokedAt,
	}
}

func parseStatusFilter(raw string) map[Status]struct{} {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := map[Status]struct{}{}
	for _, p := range parts {
		s := Status(strings.TrimSpace(p))
		if s == "" {
			continue
		}
		out[s] = struct{}{}
	}
	return out
}

// writeJSON está duplicado intencionalmente en handlers de distintos módulos
// para evitar crear paquetes/helpers compartidos demasiado pronto.
// Si más adelante se repite en más módulos, recién conviene extraerlo a un helper común.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
