package memory

import (
	"context"
	"errors"
	"sync"
	"time"

	"pet-clinical-history/internal/domain/accessgrants"
)

type grantRepo struct {
	mu   sync.RWMutex
	byID map[string]accessgrants.Grant
}

func NewAccessGrantsRepo() accessgrants.Repository {
	return &grantRepo{
		byID: make(map[string]accessgrants.Grant),
	}
}

func (r *grantRepo) Create(ctx context.Context, g accessgrants.Grant) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if g.ID == "" {
		return errors.New("grant id required")
	}
	if _, exists := r.byID[g.ID]; exists {
		return errors.New("grant already exists")
	}
	r.byID[g.ID] = g
	return nil
}

func (r *grantRepo) Update(ctx context.Context, g accessgrants.Grant) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if g.ID == "" {
		return errors.New("grant id required")
	}
	if _, exists := r.byID[g.ID]; !exists {
		return ErrNotFound
	}
	r.byID[g.ID] = g
	return nil
}

func (r *grantRepo) GetByID(ctx context.Context, id string) (accessgrants.Grant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	g, ok := r.byID[id]
	if !ok {
		return accessgrants.Grant{}, ErrNotFound
	}
	return g, nil
}

func (r *grantRepo) ListByPet(ctx context.Context, petID string) ([]accessgrants.Grant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]accessgrants.Grant, 0)
	for _, g := range r.byID {
		if g.PetID == petID {
			out = append(out, g)
		}
	}
	return out, nil
}

// Defensivo: si por data sucia existieran múltiples grants activos,
// devolvemos el más reciente por UpdatedAt (y en empate, por CreatedAt).
func (r *grantRepo) GetActiveGrant(ctx context.Context, petID, granteeUserID string) (accessgrants.Grant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var winner accessgrants.Grant
	has := false

	for _, g := range r.byID {
		if g.PetID != petID {
			continue
		}
		if g.GranteeUserID != granteeUserID {
			continue
		}
		if g.Status != accessgrants.StatusActive {
			continue
		}

		if !has {
			winner = g
			has = true
			continue
		}

		if g.UpdatedAt.After(winner.UpdatedAt) {
			winner = g
			continue
		}
		if g.UpdatedAt.Equal(winner.UpdatedAt) {
			// desempate por CreatedAt si existiera
			if g.CreatedAt.After(winner.CreatedAt) {
				winner = g
			}
		}
	}

	if !has {
		return accessgrants.Grant{}, ErrNotFound
	}
	return winner, nil
}

func (r *grantRepo) ListByGrantee(ctx context.Context, granteeUserID string) ([]accessgrants.Grant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]accessgrants.Grant, 0)
	for _, g := range r.byID {
		if g.GranteeUserID == granteeUserID {
			out = append(out, g)
		}
	}
	return out, nil
}

// (Opcional) evita import no usado de time en algunos editores si luego lo quitas.
// Ahora sí se usa en comparación de UpdatedAt (time.Time).
var _ = time.Time{}
