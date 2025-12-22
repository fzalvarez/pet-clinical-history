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
	ErrNotFound     = errors.New("not found")
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
	Species   string
	Breed     string
	Sex       string
	BirthDate *time.Time
	Notes     string
}

type UpdateProfileInput struct {
	Name      *string
	Species   *string
	Breed     *string
	Sex       *string
	BirthDate patchBirthDate // wrapper con Present + Value (*string o nil)
	Notes     *string
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

func (s *Service) UpdateProfile(ctx context.Context, petID, actorUserID string, in UpdateProfileInput) (Pet, error) {
	if strings.TrimSpace(petID) == "" || strings.TrimSpace(actorUserID) == "" {
		return Pet{}, ErrInvalidInput
	}

	p, err := s.GetByID(ctx, petID)
	if err != nil {
		return Pet{}, ErrNotFound
	}

	// Aplicar cambios PATCH
	if in.Name != nil {
		v := strings.TrimSpace(*in.Name)
		if v == "" {
			return Pet{}, ErrInvalidInput
		}
		p.Name = v
	}
	if in.Species != nil {
		p.Species = strings.TrimSpace(*in.Species)
	}
	if in.Breed != nil {
		p.Breed = strings.TrimSpace(*in.Breed)
	}
	if in.Sex != nil {
		p.Sex = strings.TrimSpace(*in.Sex)
	}
	if in.Notes != nil {
		p.Notes = strings.TrimSpace(*in.Notes)
	}

	// BirthDate: si estuvo presente, puede ser nil (limpiar) o string YYYY-MM-DD
	if in.BirthDate.Present {
		if in.BirthDate.Value == nil {
			p.BirthDate = nil
		} else {
			raw := strings.TrimSpace(*in.BirthDate.Value)
			if raw == "" {
				return Pet{}, ErrInvalidInput
			}
			t, err := time.Parse("2006-01-02", raw)
			if err != nil {
				return Pet{}, ErrInvalidInput
			}
			p.BirthDate = &t
		}
	}

	p.UpdatedAt = time.Now()

	// Persistir
	if err := s.repo.Update(ctx, p); err != nil {
		return Pet{}, err
	}

	return p, nil
}
