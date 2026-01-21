package pets

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrPetInvalidInput = errors.New("invalid input")
	ErrPetNotFound     = errors.New("not found")
)

// Service agrupa casos de uso del dominio Pets.
// Nota de consistencia: los casos de uso deben preferir s.now() (en lugar de time.Now())
// para facilitar pruebas (mock del tiempo) y mantener el mismo patrón que otros módulos.
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
	Species   Species
	Breed     string
	Sex       Sex
	BirthDate *time.Time
	Notes     string
}

func (s *Service) Create(ctx context.Context, ownerUserID string, in CreateInput) (Pet, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" {
		return Pet{}, ErrPetInvalidInput
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return Pet{}, ErrPetInvalidInput
	}

	now := s.now()

	p := Pet{
		ID:          uuid.NewString(),
		OwnerUserID: ownerUserID,
		Name:        name,
		Species:     Species(strings.TrimSpace(string(in.Species))),
		Breed:       strings.TrimSpace(in.Breed),
		Sex:         Sex(strings.TrimSpace(string(in.Sex))),
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
	id = strings.TrimSpace(id)
	if id == "" {
		return Pet{}, ErrPetInvalidInput
	}
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Pet{}, err
	}
	return p, nil
}

func (s *Service) ListByOwner(ctx context.Context, ownerUserID string) ([]Pet, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" {
		return nil, ErrPetInvalidInput
	}
	return s.repo.ListByOwner(ctx, ownerUserID)
}

// BirthDatePatch permite PATCH real diferenciando:
// - Present=false: no tocar
// - Present=true y Value=nil: limpiar
// - Present=true y Value!=nil: parsear YYYY-MM-DD
type BirthDatePatch struct {
	Present bool
	Value   *string
}

type UpdateProfileInput struct {
	Name      *string
	Species   *Species
	Breed     *string
	Sex       *Sex
	BirthDate BirthDatePatch
	Notes     *string
}

func (s *Service) UpdateProfile(ctx context.Context, petID string, in UpdateProfileInput) (Pet, error) {
	petID = strings.TrimSpace(petID)
	if petID == "" {
		return Pet{}, ErrPetInvalidInput
	}

	p, err := s.repo.GetByID(ctx, petID)
	if err != nil {
		return Pet{}, ErrPetNotFound
	}

	if in.Name != nil {
		v := strings.TrimSpace(*in.Name)
		if v == "" {
			return Pet{}, ErrPetInvalidInput
		}
		p.Name = v
	}
	if in.Species != nil {
		p.Species = Species(strings.TrimSpace(string(*in.Species)))
	}
	if in.Breed != nil {
		p.Breed = strings.TrimSpace(*in.Breed)
	}
	if in.Sex != nil {
		p.Sex = Sex(strings.TrimSpace(string(*in.Sex)))
	}
	if in.Notes != nil {
		p.Notes = strings.TrimSpace(*in.Notes)
	}

	if in.BirthDate.Present {
		if in.BirthDate.Value == nil {
			p.BirthDate = nil
		} else {
			raw := strings.TrimSpace(*in.BirthDate.Value)
			if raw == "" {
				return Pet{}, ErrPetInvalidInput
			}
			t, err := time.Parse("2006-01-02", raw)
			if err != nil {
				return Pet{}, ErrPetInvalidInput
			}
			p.BirthDate = &t
		}
	}

	p.UpdatedAt = s.now()

	if err := s.repo.Update(ctx, p); err != nil {
		return Pet{}, err
	}
	return p, nil
}
