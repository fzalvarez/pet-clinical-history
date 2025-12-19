package events

import (
	"context"
	"time"
)

type Repository interface {
	Create(ctx context.Context, e PetEvent) error
	GetByID(ctx context.Context, id string) (PetEvent, error)
	ListByPet(ctx context.Context, petID string, filter ListFilter) ([]PetEvent, error)
	Void(ctx context.Context, id string) error
}

type ListFilter struct {
	Types []EventType
	From  *time.Time
	To    *time.Time
	Query string
	Limit int
}
