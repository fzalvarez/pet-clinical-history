package pets

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
	Name      string
	Species   string
	Breed     string
	Sex       string
	BirthDate *time.Time
	Notes     string
}

func (s *Service) Create(ctx context.Context, ownerUserID string, in CreateInput) (Pet, error) {
	if strings.TrimSpace(ownerUserID) == "" {
		return Pet{}, ErrInvalidInput
	}
	if strings.TrimSpace(in.Name) == "" {
		return Pet{}, ErrInvalidInput
	}
	if strings.TrimSpace(in.Species) == "" {
		return Pet{}, ErrInvalidInput
	}

	now := s.now()
	p := Pet{
		ID:          uuid.NewString(),
		OwnerUserID: ownerUserID,
		Name:        strings.TrimSpace(in.Name),
		Species:     strings.TrimSpace(in.Species),
		Breed:       strings.TrimSpace(in.Breed),
		Sex:         strings.TrimSpace(in.Sex),
		BirthDate:   in.BirthDate,
		Notes:       strings.TrimSpace(in.Notes),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.repo.Create(ctx, p); err != nil {
		return Pet{}, err
	}
	return p, nil
}

func (s *Service) GetByID(ctx context.Context, id string) (Pet, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) ListByOwner(ctx context.Context, ownerUserID string) ([]Pet, error) {
	return s.repo.ListByOwner(ctx, ownerUserID)
}
