package memory

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"

	"pet-clinical-history/internal/domain/pets"
)

var (
	ErrNotFound = errors.New("not found")
)

type petRepo struct {
	mu   sync.RWMutex
	byID map[string]pets.Pet
}

func NewPetRepo() pets.Repository {
	return &petRepo{
		byID: make(map[string]pets.Pet),
	}
}

func (r *petRepo) Create(ctx context.Context, p pets.Pet) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if strings.TrimSpace(p.ID) == "" {
		return errors.New("pet id required")
	}
	if _, exists := r.byID[p.ID]; exists {
		return errors.New("pet already exists")
	}
	r.byID[p.ID] = p
	return nil
}

func (r *petRepo) Update(ctx context.Context, p pets.Pet) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if strings.TrimSpace(p.ID) == "" {
		return errors.New("pet id required")
	}
	if _, exists := r.byID[p.ID]; !exists {
		return ErrNotFound
	}
	r.byID[p.ID] = p
	return nil
}

func (r *petRepo) GetByID(ctx context.Context, id string) (pets.Pet, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.byID[id]
	if !ok {
		return pets.Pet{}, ErrNotFound
	}
	return p, nil
}

func (r *petRepo) ListByOwner(ctx context.Context, ownerUserID string) ([]pets.Pet, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]pets.Pet, 0)
	for _, p := range r.byID {
		if p.OwnerUserID == ownerUserID {
			out = append(out, p)
		}
	}

	// Orden estable por created_at asc (solo para consistencia en dev)
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})

	return out, nil
}
