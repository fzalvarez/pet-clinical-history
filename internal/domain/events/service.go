package events

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidInput = errors.New("invalid input")
)

type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository) *Service {
	return &Service{
		repo: repo,
		now:  time.Now,
	}
}

type CreateInput struct {
	Type       EventType
	OccurredAt time.Time
	Title      string
	Notes      string
	Source     Source
	Visibility Visibility
}

func (s *Service) Create(ctx context.Context, petID string, actor Actor, in CreateInput) (PetEvent, error) {
	if strings.TrimSpace(petID) == "" {
		return PetEvent{}, ErrInvalidInput
	}
	if in.Type == "" {
		return PetEvent{}, ErrInvalidInput
	}
	if in.OccurredAt.IsZero() {
		return PetEvent{}, ErrInvalidInput
	}
	if actor.Type == "" || strings.TrimSpace(actor.ID) == "" {
		return PetEvent{}, ErrInvalidInput
	}

	now := s.now()

	src := in.Source
	if src == "" {
		src = SourceManual
	}
	vis := in.Visibility
	if vis == "" {
		vis = VisibilityShared
	}

	e := PetEvent{
		ID:         uuid.NewString(),
		PetID:      petID,
		Type:       in.Type,
		OccurredAt: in.OccurredAt,
		RecordedAt: now,
		Title:      strings.TrimSpace(in.Title),
		Notes:      strings.TrimSpace(in.Notes),
		Actor:      actor,
		Source:     src,
		Visibility: vis,
		Status:     EventStatusActive,
	}

	if err := s.repo.Create(ctx, e); err != nil {
		return PetEvent{}, err
	}
	return e, nil
}

func (s *Service) GetByID(ctx context.Context, id string) (PetEvent, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return PetEvent{}, ErrInvalidInput
	}
	return s.repo.GetByID(ctx, id)
}

func (s *Service) ListByPet(ctx context.Context, petID string, filter ListFilter) ([]PetEvent, error) {
	return s.repo.ListByPet(ctx, petID, filter)
}

// Void marca el evento como voided (no se borra).
func (s *Service) Void(ctx context.Context, id string) (PetEvent, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return PetEvent{}, ErrInvalidInput
	}
	if err := s.repo.Void(ctx, id); err != nil {
		return PetEvent{}, err
	}
	return s.repo.GetByID(ctx, id)
}
