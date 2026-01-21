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
	r.Route("/pets", func(pr chi.Router) {
		pr.Post("/", createPetHandler(svc))
		pr.Get("/", listPetsHandler(svc))

		// Perfil de mascota (owner o delegado con pet:read)
		pr.Get("/{petID}", getPetHandler(svc, grantsSvc))

		// Editar perfil (owner o delegado con pet:edit_profile)
		pr.Patch("/{petID}", updatePetHandler(svc, grantsSvc))
	})

	// Mascotas compartidas conmigo (delegado)
	r.Get("/me/pets", listMySharedPetsHandler(svc, grantsSvc))
}

// createPetRequest es el cuerpo de la solicitud para crear una nueva mascota.
type createPetRequest struct {
	Name      string  `json:"name"`
	Species   Species `json:"species" enums:"dog,cat"` // dog, cat
	Breed     string  `json:"breed"`                   // Ej para dog: labrador, poodle. Ej para cat: persian, common.
	Sex       Sex     `json:"sex" enums:"male,female,unknown"`
	BirthDate string  `json:"birth_date"` // YYYY-MM-DD opcional
	Notes     string  `json:"notes"`
}

// updatePetRequest es el cuerpo parcial para actualizar el perfil de una mascota.
type updatePetRequest struct {
	Name    *string  `json:"name"`
	Species *Species `json:"species" enums:"dog,cat"`
	Breed   *string  `json:"breed"`
	Sex     *Sex     `json:"sex" enums:"male,female,unknown"`
	Notes   *string  `json:"notes"`
	// birth_date se decodifica aparte para soportar null
}

// petResponse representa el perfil público de una mascota devuelto por la API.
type petResponse struct {
	ID          string     `json:"id"`
	OwnerUserID string     `json:"owner_user_id"`
	Name        string     `json:"name"`
	Species     Species    `json:"species"`
	Breed       string     `json:"breed"`
	Sex         Sex        `json:"sex"`
	BirthDate   *time.Time `json:"birth_date,omitempty"`
	Notes       string     `json:"notes"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// sharedPetResponse representa una mascota compartida con el usuario autenticado.
type sharedPetResponse struct {
	Pet    petResponse          `json:"pet"`
	Grant  grantMini            `json:"grant"`
	Scopes []accessgrants.Scope `json:"scopes"`
}

// grantMini resume un grant asociado a una mascota compartida.
type grantMini struct {
	ID     string              `json:"id"`
	Status accessgrants.Status `json:"status"`
}

// createPetHandler godoc
// @Summary Crear una mascota
// @Description Crea una mascota. Species: `dog`, `cat`. Breeds Perro: `labrador`, `golden_retriever`, `poodle`, etc. Breeds Gato: `persian`, `common`, etc.
// @Tags pets
// @Accept json
// @Produce json
// @Param X-Debug-User-ID header string false "Solo en modo dev, ID de usuario para depuración"
// @Param Authorization header string false "Bearer token en producción"
// @Param payload body createPetRequest true "Datos de la mascota; birth_date opcional (YYYY-MM-DD)"
// @Success 201 {object} petResponse
// @Failure 401 {string} string "unauthorized"
// @Router /pets [post]
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

// listPetsHandler godoc
// @Summary Listar mis mascotas
// @Description Lista todas las mascotas cuyo propietario es el usuario autenticado. Autenticación: `X-Debug-User-ID` (dev) o `Authorization: Bearer <token>` (prod). Solo el owner ve este listado; los delegados no listan aquí.
// @Tags pets
// @Produce json
// @Param X-Debug-User-ID header string false "Solo en modo dev, ID de usuario para depuración"
// @Param Authorization header string false "Bearer token en producción"
// @Success 200 {array} petResponse
// @Failure 401 {string} string "unauthorized"
// @Router /pets [get]
func listPetsHandler(svc *Service) http.HandlerFunc {
	// Owner-only
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

// getPetHandler godoc
// @Summary Obtener perfil de mascota
// @Description Obtiene el perfil de una mascota. El dueño siempre tiene acceso (bypass). Un delegado necesita un grant activo con scope `pet:read`. Autenticación: `X-Debug-User-ID` (dev) o `Authorization: Bearer <token>` (prod).
// @Tags pets
// @Produce json
// @Param X-Debug-User-ID header string false "Solo en modo dev, ID de usuario para depuración"
// @Param Authorization header string false "Bearer token en producción"
// @Param petID path string true "ID de la mascota"
// @Success 200 {object} petResponse
// @Failure 403 {string} string "forbidden"
// @Failure 404 {string} string "pet not found"
// @Router /pets/{petID} [get]
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

// updatePetHandler godoc
// @Summary Actualizar perfil de mascota
// @Description Actualiza parcialmente el perfil de una mascota. El dueño siempre tiene acceso (bypass). Un delegado necesita un grant activo con scope `pet:edit_profile`. Autenticación: `X-Debug-User-ID` (dev) o `Authorization: Bearer <token>` (prod). Campo `birth_date` se maneja con semántica PATCH especial: si no se envía, no cambia; si se envía como `null`, se limpia; si se envía como string `YYYY-MM-DD`, se actualiza.
// @Tags pets
// @Accept json
// @Produce json
// @Param X-Debug-User-ID header string false "Solo en modo dev, ID de usuario para depuración"
// @Param Authorization header string false "Bearer token en producción"
// @Param petID path string true "ID de la mascota"
// @Param payload body updatePetRequest true "Campos a actualizar"
// @Success 200 {object} petResponse
// @Failure 403 {string} string "forbidden"
// @Router /pets/{petID} [patch]
func updatePetHandler(svc *Service, grantsSvc *accessgrants.Service) http.HandlerFunc {
	// Owner bypass, delegado requiere pet:edit_profile
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.GetClaims(r.Context())
		if !ok || strings.TrimSpace(claims.UserID) == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		petID := chi.URLParam(r, "petID")

		// Verifica existencia + ownership para auth
		p, err := svc.GetByID(r.Context(), petID)
		if err != nil {
			http.Error(w, "pet not found", http.StatusNotFound)
			return
		}

		if p.OwnerUserID != claims.UserID {
			g, err := grantsSvc.GetActiveGrant(r.Context(), petID, claims.UserID)
			if err != nil || !accessgrants.HasScope(g, accessgrants.ScopePetEditProfile) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
		}

		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()

		// Para soportar birth_date: null, detectamos presencia en raw map
		var raw map[string]json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		// Parse base fields
		var req updatePetRequest
		{
			b, _ := json.Marshal(raw)
			if err := json.Unmarshal(b, &req); err != nil {
				http.Error(w, "invalid json", http.StatusBadRequest)
				return
			}
		}

		// birth_date patch
		bdp := BirthDatePatch{Present: false, Value: nil}
		if v, ok := raw["birth_date"]; ok {
			bdp.Present = true
			if string(v) == "null" {
				bdp.Value = nil
			} else {
				var s string
				if err := json.Unmarshal(v, &s); err != nil {
					http.Error(w, "birth_date must be YYYY-MM-DD or null", http.StatusBadRequest)
					return
				}
				bdp.Value = &s
			}
		}

		updated, err := svc.UpdateProfile(r.Context(), petID, UpdateProfileInput{
			Name:      req.Name,
			Species:   req.Species,
			Breed:     req.Breed,
			Sex:       req.Sex,
			Notes:     req.Notes,
			BirthDate: bdp,
		})
		if err != nil {
			switch err {
			case ErrPetInvalidInput:
				http.Error(w, err.Error(), http.StatusBadRequest)
			case ErrPetNotFound:
				http.Error(w, "pet not found", http.StatusNotFound)
			default:
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
			return
		}

		writeJSON(w, http.StatusOK, toPetResponse(updated))
	}
}

// listMySharedPetsHandler godoc
// @Summary Listar mascotas compartidas conmigo
// @Description Lista las mascotas compartidas con el usuario autenticado mediante grants activos que incluyan el scope `pet:read`. Autenticación: `X-Debug-User-ID` (dev) o `Authorization: Bearer <token>` (prod).
// @Tags pets
// @Produce json
// @Param X-Debug-User-ID header string false "Solo en modo dev, ID de usuario para depuración"
// @Param Authorization header string false "Bearer token en producción"
// @Success 200 {array} sharedPetResponse
// @Router /me/pets [get]
func listMySharedPetsHandler(svc *Service, grantsSvc *accessgrants.Service) http.HandlerFunc {
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
			// Para mostrar perfil, exigimos pet:read
			if !accessgrants.HasScope(g, accessgrants.ScopePetRead) {
				continue
			}
			if _, ok := seen[g.PetID]; ok {
				continue
			}
			seen[g.PetID] = struct{}{}

			p, err := svc.GetByID(r.Context(), g.PetID)
			if err != nil {
				continue
			}

			out = append(out, sharedPetResponse{
				Pet: toPetResponse(p),
				Grant: grantMini{
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

// writeJSON está duplicado intencionalmente en handlers de distintos módulos (pets/events/accessgrants)
// para evitar crear paquetes/helpers compartidos demasiado pronto.
// Si más adelante se repite en más módulos, recién conviene extraerlo a un helper común.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
