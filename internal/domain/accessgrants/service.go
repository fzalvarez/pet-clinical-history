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
	petID := strings.TrimSpace(in.PetID)
	ownerID := strings.TrimSpace(in.OwnerUserID)
	granteeID := strings.TrimSpace(in.GranteeUserID)

	if petID == "" || ownerID == "" || granteeID == "" {
		return Grant{}, ErrInvalidInput
	}
	if ownerID == granteeID {
		return Grant{}, ErrInvalidInput
	}

	// Scopes: si viene vacío, aplicamos default útil (ver perfil + ver timeline).
	// Si viene con valores, los validamos estrictamente.
	var scopes []Scope
	var err error
	if len(in.Scopes) == 0 {
		scopes = []Scope{ScopePetRead, ScopeEventsRead}
	} else {
		scopes, err = normalizeScopesStrict(in.Scopes)
		if err != nil {
			return Grant{}, err
		}
		if len(scopes) == 0 {
			return Grant{}, ErrInvalidInput
		}
	}

	now := s.now()

	// 1) Buscar grants existentes para (petID, granteeID, ownerID)
	existing, allMatches, err := s.findLatestMatch(ctx, petID, ownerID, granteeID)
	if err == nil && existing.ID != "" {
		// Si el "winner" está revoked, permitimos re-invitar creando uno nuevo.
		if existing.Status != StatusRevoked {
			// 2) Deduplicar: revocar cualquier otro matching grant no-revoked
			_ = s.revokeOtherMatches(ctx, existing.ID, allMatches, now)

			// 3) Actualizar scopes del winner (permite "cambiar" scopes sin endpoint adicional)
			existing.Scopes = scopes
			existing.UpdatedAt = now

			if err := s.repo.Update(ctx, existing); err != nil {
				return Grant{}, err
			}
			return existing, nil
		}
	}

	// Crear nuevo invite
	g := Grant{
		ID:            uuid.NewString(),
		PetID:         petID,
		OwnerUserID:   ownerID,
		GranteeUserID: granteeID,
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
	grantID = strings.TrimSpace(grantID)
	granteeUserID = strings.TrimSpace(granteeUserID)

	if grantID == "" || granteeUserID == "" {
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
	grantID = strings.TrimSpace(grantID)
	ownerUserID = strings.TrimSpace(ownerUserID)

	if grantID == "" || ownerUserID == "" {
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
	petID = strings.TrimSpace(petID)
	if petID == "" {
		return nil, ErrInvalidInput
	}
	return s.repo.ListByPet(ctx, petID)
}

func (s *Service) GetActiveGrant(ctx context.Context, petID, granteeUserID string) (Grant, error) {
	petID = strings.TrimSpace(petID)
	granteeUserID = strings.TrimSpace(granteeUserID)

	if petID == "" || granteeUserID == "" {
		return Grant{}, ErrInvalidInput
	}
	g, err := s.repo.GetActiveGrant(ctx, petID, granteeUserID)
	if err != nil {
		return Grant{}, ErrNotFound
	}
	return g, nil
}

func (s *Service) ListByGrantee(ctx context.Context, granteeUserID string) ([]Grant, error) {
	granteeUserID = strings.TrimSpace(granteeUserID)
	if granteeUserID == "" {
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

func (s *Service) findLatestMatch(ctx context.Context, petID, ownerID, granteeID string) (Grant, []Grant, error) {
	items, err := s.repo.ListByPet(ctx, petID)
	if err != nil {
		return Grant{}, nil, err
	}

	matches := make([]Grant, 0)
	var winner Grant
	hasWinner := false

	for _, g := range items {
		if g.PetID != petID || g.OwnerUserID != ownerID || g.GranteeUserID != granteeID {
			continue
		}
		matches = append(matches, g)

		if !hasWinner || g.UpdatedAt.After(winner.UpdatedAt) {
			winner = g
			hasWinner = true
		}
	}

	if !hasWinner {
		return Grant{}, matches, ErrNotFound
	}
	return winner, matches, nil
}

func (s *Service) revokeOtherMatches(ctx context.Context, winnerID string, matches []Grant, now time.Time) error {
	for _, g := range matches {
		if g.ID == "" || g.ID == winnerID {
			continue
		}
		if g.Status == StatusRevoked {
			continue
		}
		g.Status = StatusRevoked
		g.UpdatedAt = now
		g.RevokedAt = &now
		_ = s.repo.Update(ctx, g) // best-effort (MVP)
	}
	return nil
}

func normalizeScopesStrict(in []Scope) ([]Scope, error) {
	allowed := map[Scope]struct{}{
		ScopePetRead:        {},
		ScopePetEditProfile: {},
		ScopeEventsRead:     {},
		ScopeEventsCreate:   {},
		ScopeEventsVoid:     {},
		ScopeAttachmentsAdd: {},
	}

	seen := map[Scope]struct{}{}
	out := make([]Scope, 0, len(in))

	for _, raw := range in {
		s := Scope(strings.TrimSpace(string(raw)))
		if s == "" {
			continue
		}
		if _, ok := allowed[s]; !ok {
			return nil, ErrInvalidInput
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}

	return out, nil
}
