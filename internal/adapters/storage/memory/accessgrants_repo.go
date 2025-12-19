package memory

import (
	"context"
	"errors"
	"sync"

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

func (r *grantRepo) GetActiveGrant(ctx context.Context, petID, granteeUserID string) (accessgrants.Grant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, g := range r.byID {
		if g.PetID == petID &&
			g.GranteeUserID == granteeUserID &&
			g.Status == accessgrants.StatusActive {
			return g, nil
		}
	}
	return accessgrants.Grant{}, ErrNotFound
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
