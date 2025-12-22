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

	// Scopes:
	// - Si viene vacío: default útil (ver perfil + ver timeline)
	// - Si viene con valores: validación estricta (solo scopes soportados)
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

	now := s.now()

	// Idempotente
	if g.Status == StatusActive {
		// defensivo: garantizar "solo un activo" para el mismo pet+grantee
		_ = s.revokeOtherByPetAndGrantee(ctx, g.ID, g.PetID, g.GranteeUserID, now)
		return g, nil
	}
	if g.Status != StatusInvited {
		return Grant{}, ErrBadState
	}

	g.Status = StatusActive
	g.UpdatedAt = now

	if err := s.repo.Update(ctx, g); err != nil {
		return Grant{}, err
	}

	// Cierra loop: al activar uno, revoca cualquier otro grant no-revocado para el mismo pet+grantee.
	_ = s.revokeOtherByPetAndGrantee(ctx, g.ID, g.PetID, g.GranteeUserID, now)

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

// revokeOtherByPetAndGrantee revoca best-effort cualquier otro grant no revocado para (petID, granteeID),
// excepto keepID. Esto evita múltiples "activos" para el mismo delegado.
func (s *Service) revokeOtherByPetAndGrantee(ctx context.Context, keepID, petID, granteeID string, now time.Time) error {
	items, err := s.repo.ListByPet(ctx, petID)
	if err != nil {
		return err
	}

	for _, g := range items {
		if g.ID == "" || g.ID == keepID {
			continue
		}
		if g.PetID != petID || g.GranteeUserID != granteeID {
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
