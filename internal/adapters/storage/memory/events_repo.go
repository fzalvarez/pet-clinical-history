package memory

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"pet-clinical-history/internal/domain/events"
)

type eventRepo struct {
	mu   sync.RWMutex
	byID map[string]events.PetEvent
}

func NewEventRepo() events.Repository {
	return &eventRepo{
		byID: make(map[string]events.PetEvent),
	}
}

func (r *eventRepo) Create(ctx context.Context, e events.PetEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if e.ID == "" {
		return errors.New("event id required")
	}
	if _, exists := r.byID[e.ID]; exists {
		return errors.New("event already exists")
	}

	r.byID[e.ID] = e
	return nil
}

func (r *eventRepo) GetByID(ctx context.Context, id string) (events.PetEvent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	e, ok := r.byID[id]
	if !ok {
		return events.PetEvent{}, ErrNotFound
	}
	return e, nil
}

func (r *eventRepo) ListByPet(ctx context.Context, petID string, filter events.ListFilter) ([]events.PetEvent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}

	out := make([]events.PetEvent, 0)

	for _, e := range r.byID {
		if e.PetID != petID {
			continue
		}

		// Type filter
		if len(filter.Types) > 0 {
			ok := false
			for _, t := range filter.Types {
				if e.Type == t {
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
		}

		// Date filters (occurred_at)
		if filter.From != nil {
			if e.OccurredAt.Before((*filter.From).Add(-1 * time.Nanosecond)) {
				continue
			}
		}
		if filter.To != nil {
			if e.OccurredAt.After(*filter.To) {
				continue
			}
		}

		// Query filter
		if q := strings.TrimSpace(filter.Query); q != "" {
			hay := strings.ToLower(e.Title + " " + e.Notes)
			if !strings.Contains(hay, strings.ToLower(q)) {
				continue
			}
		}

		out = append(out, e)
	}

	// Orden por occurred_at desc (más reciente primero)
	sort.Slice(out, func(i, j int) bool {
		return out[i].OccurredAt.After(out[j].OccurredAt)
	})

	if len(out) > limit {
		out = out[:limit]
	}

	return out, nil
}

func (r *eventRepo) Void(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	e, ok := r.byID[id]
	if !ok {
		return ErrNotFound
	}
	e.Status = events.EventStatusVoided
	r.byID[id] = e
	return nil
}
