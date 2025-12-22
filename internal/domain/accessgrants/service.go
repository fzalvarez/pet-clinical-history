package accessgrants

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrForbidden    = errors.New("forbidden")
	ErrNotFound     = errors.New("not found")
	ErrBadState     = errors.New("invalid state")
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

type InviteInput struct {
	PetID         string
	OwnerUserID   string
	GranteeUserID string
	Scopes        []Scope
}

func (s *Service) Invite(ctx context.Context, in InviteInput) (Grant, error) {
	if strings.TrimSpace(in.PetID) == "" ||
		strings.TrimSpace(in.OwnerUserID) == "" ||
		strings.TrimSpace(in.GranteeUserID) == "" {
		return Grant{}, ErrInvalidInput
	}
	if in.OwnerUserID == in.GranteeUserID {
		return Grant{}, ErrInvalidInput
	}

	scopes := normalizeScopes(in.Scopes)
	if len(scopes) == 0 {
		// Default mínimo útil para que el delegado pueda ver perfil + timeline
		scopes = []Scope{ScopePetRead, ScopeEventsRead}
	}

	now := s.now()

	g := Grant{
		ID:            uuid.NewString(),
		PetID:         in.PetID,
		OwnerUserID:   in.OwnerUserID,
		GranteeUserID: in.GranteeUserID,
		Scopes:        scopes,
		Status:        StatusInvited,
		CreatedAt:     now,
		UpdatedAt:     now,
		RevokedAt:     nil,
	}

	if err := s.repo.Create(ctx, g); err != nil {
		return Grant{}, err
	}
	return g, nil
}

func (s *Service) Accept(ctx context.Context, grantID, granteeUserID string) (Grant, error) {
	if strings.TrimSpace(grantID) == "" || strings.TrimSpace(granteeUserID) == "" {
		return Grant{}, ErrInvalidInput
	}

	g, err := s.repo.GetByID(ctx, grantID)
	if err != nil {
		return Grant{}, ErrNotFound
	}

	if g.GranteeUserID != granteeUserID {
		return Grant{}, ErrForbidden
	}
	if g.Status == StatusRevoked {
		return Grant{}, ErrBadState
	}

	// Idempotente
	if g.Status == StatusActive {
		return g, nil
	}
	if g.Status != StatusInvited {
		return Grant{}, ErrBadState
	}

	now := s.now()
	g.Status = StatusActive
	g.UpdatedAt = now

	if err := s.repo.Update(ctx, g); err != nil {
		return Grant{}, err
	}
	return g, nil
}

func (s *Service) Revoke(ctx context.Context, grantID, ownerUserID string) (Grant, error) {
	if strings.TrimSpace(grantID) == "" || strings.TrimSpace(ownerUserID) == "" {
		return Grant{}, ErrInvalidInput
	}

	g, err := s.repo.GetByID(ctx, grantID)
	if err != nil {
		return Grant{}, ErrNotFound
	}

	if g.OwnerUserID != ownerUserID {
		return Grant{}, ErrForbidden
	}

	// Idempotente
	if g.Status == StatusRevoked {
		return g, nil
	}

	now := s.now()
	g.Status = StatusRevoked
	g.UpdatedAt = now
	g.RevokedAt = &now

	if err := s.repo.Update(ctx, g); err != nil {
		return Grant{}, err
	}
	return g, nil
}

func (s *Service) ListByPet(ctx context.Context, petID string) ([]Grant, error) {
	if strings.TrimSpace(petID) == "" {
		return nil, ErrInvalidInput
	}
	return s.repo.ListByPet(ctx, petID)
}

func (s *Service) GetActiveGrant(ctx context.Context, petID, granteeUserID string) (Grant, error) {
	if strings.TrimSpace(petID) == "" || strings.TrimSpace(granteeUserID) == "" {
		return Grant{}, ErrInvalidInput
	}
	g, err := s.repo.GetActiveGrant(ctx, petID, granteeUserID)
	if err != nil {
		return Grant{}, ErrNotFound
	}
	return g, nil
}

func (s *Service) ListByGrantee(ctx context.Context, granteeUserID string) ([]Grant, error) {
	if strings.TrimSpace(granteeUserID) == "" {
		return nil, ErrInvalidInput
	}
	return s.repo.ListByGrantee(ctx, granteeUserID)
}

// HasScope valida si el grant incluye un scope.
func HasScope(g Grant, scope Scope) bool {
	for _, s := range g.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

func normalizeScopes(in []Scope) []Scope {
	seen := map[Scope]struct{}{}
	out := make([]Scope, 0, len(in))
	for _, s := range in {
		s = Scope(strings.TrimSpace(string(s)))
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
