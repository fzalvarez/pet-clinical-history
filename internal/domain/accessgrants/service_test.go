package accessgrants

import (
	"context"
	"errors"
	"testing"
	"time"
)

// -------------------------
// Test repo (in-memory)
// -------------------------

var errRepoNotFound = errors.New("repo: not found")

type testRepo struct {
	byID map[string]Grant
}

func newTestRepo() *testRepo {
	return &testRepo{byID: map[string]Grant{}}
}

func (r *testRepo) Create(ctx context.Context, g Grant) error {
	if g.ID == "" {
		return errors.New("repo: id required")
	}
	if _, ok := r.byID[g.ID]; ok {
		return errors.New("repo: already exists")
	}
	r.byID[g.ID] = g
	return nil
}

func (r *testRepo) Update(ctx context.Context, g Grant) error {
	if g.ID == "" {
		return errors.New("repo: id required")
	}
	if _, ok := r.byID[g.ID]; !ok {
		return errRepoNotFound
	}
	r.byID[g.ID] = g
	return nil
}

func (r *testRepo) GetByID(ctx context.Context, id string) (Grant, error) {
	g, ok := r.byID[id]
	if !ok {
		return Grant{}, errRepoNotFound
	}
	return g, nil
}

func (r *testRepo) ListByPet(ctx context.Context, petID string) ([]Grant, error) {
	out := make([]Grant, 0)
	for _, g := range r.byID {
		if g.PetID == petID {
			out = append(out, g)
		}
	}
	return out, nil
}

func (r *testRepo) GetActiveGrant(ctx context.Context, petID, granteeUserID string) (Grant, error) {
	var winner Grant
	has := false

	for _, g := range r.byID {
		if g.PetID != petID {
			continue
		}
		if g.GranteeUserID != granteeUserID {
			continue
		}
		if g.Status != StatusActive {
			continue
		}

		if !has {
			winner = g
			has = true
			continue
		}
		if g.UpdatedAt.After(winner.UpdatedAt) {
			winner = g
			continue
		}
		if g.UpdatedAt.Equal(winner.UpdatedAt) && g.CreatedAt.After(winner.CreatedAt) {
			winner = g
		}
	}

	if !has {
		return Grant{}, errRepoNotFound
	}
	return winner, nil
}

func (r *testRepo) ListByGrantee(ctx context.Context, granteeUserID string) ([]Grant, error) {
	out := make([]Grant, 0)
	for _, g := range r.byID {
		if g.GranteeUserID == granteeUserID {
			out = append(out, g)
		}
	}
	return out, nil
}

// -------------------------
// Tests
// -------------------------

func TestService_Invite_DefaultScopes_WhenEmpty(t *testing.T) {
	repo := newTestRepo()
	svc := NewService(repo)

	now := time.Date(2025, 12, 22, 10, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	g, err := svc.Invite(context.Background(), InviteInput{
		PetID:         "pet-1",
		OwnerUserID:   "owner-1",
		GranteeUserID: "delegate-1",
		Scopes:        nil,
	})
	if err != nil {
		t.Fatalf("Invite returned error: %v", err)
	}
	if g.Status != StatusInvited {
		t.Fatalf("expected status invited, got %s", g.Status)
	}
	if g.CreatedAt != now || g.UpdatedAt != now {
		t.Fatalf("expected CreatedAt/UpdatedAt to be now")
	}
	// default: pet:read + events:read
	if !HasScope(g, ScopePetRead) || !HasScope(g, ScopeEventsRead) {
		t.Fatalf("expected default scopes pet:read + events:read, got %#v", g.Scopes)
	}
}

func TestService_Invite_StrictScopes_RejectsUnknown(t *testing.T) {
	repo := newTestRepo()
	svc := NewService(repo)

	_, err := svc.Invite(context.Background(), InviteInput{
		PetID:         "pet-1",
		OwnerUserID:   "owner-1",
		GranteeUserID: "delegate-1",
		Scopes:        []Scope{ScopeEventsRead, Scope("bad:scope")},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if err != ErrInvalidInput {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestService_Invite_Dedup_UpdatesSameGrant(t *testing.T) {
	repo := newTestRepo()
	svc := NewService(repo)

	now1 := time.Date(2025, 12, 22, 10, 0, 0, 0, time.UTC)
	now2 := now1.Add(5 * time.Minute)

	svc.now = func() time.Time { return now1 }
	g1, err := svc.Invite(context.Background(), InviteInput{
		PetID:         "pet-1",
		OwnerUserID:   "owner-1",
		GranteeUserID: "delegate-1",
		Scopes:        []Scope{ScopeEventsRead},
	})
	if err != nil {
		t.Fatalf("Invite #1 error: %v", err)
	}

	svc.now = func() time.Time { return now2 }
	g2, err := svc.Invite(context.Background(), InviteInput{
		PetID:         "pet-1",
		OwnerUserID:   "owner-1",
		GranteeUserID: "delegate-1",
		Scopes:        []Scope{ScopeEventsRead, ScopeEventsCreate},
	})
	if err != nil {
		t.Fatalf("Invite #2 error: %v", err)
	}

	if g2.ID != g1.ID {
		t.Fatalf("expected same grant ID (dedup), got %s vs %s", g1.ID, g2.ID)
	}
	if g2.UpdatedAt != now2 {
		t.Fatalf("expected UpdatedAt to change on reinvite")
	}
	if !HasScope(g2, ScopeEventsCreate) || !HasScope(g2, ScopeEventsRead) {
		t.Fatalf("expected scopes updated, got %#v", g2.Scopes)
	}
}

func TestService_Accept_SetsActive_AndIdempotent(t *testing.T) {
	repo := newTestRepo()
	svc := NewService(repo)

	now1 := time.Date(2025, 12, 22, 10, 0, 0, 0, time.UTC)
	now2 := now1.Add(2 * time.Minute)

	svc.now = func() time.Time { return now1 }
	g, err := svc.Invite(context.Background(), InviteInput{
		PetID:         "pet-1",
		OwnerUserID:   "owner-1",
		GranteeUserID: "delegate-1",
	})
	if err != nil {
		t.Fatalf("Invite error: %v", err)
	}

	svc.now = func() time.Time { return now2 }
	accepted, err := svc.Accept(context.Background(), g.ID, "delegate-1")
	if err != nil {
		t.Fatalf("Accept error: %v", err)
	}
	if accepted.Status != StatusActive {
		t.Fatalf("expected active, got %s", accepted.Status)
	}

	// idempotente
	accepted2, err := svc.Accept(context.Background(), g.ID, "delegate-1")
	if err != nil {
		t.Fatalf("Accept #2 error: %v", err)
	}
	if accepted2.Status != StatusActive {
		t.Fatalf("expected active after idempotent accept, got %s", accepted2.Status)
	}
}

func TestService_Accept_LeavesOnlyOneActive_ForPetAndGrantee(t *testing.T) {
	// Este test valida el “loop”:
	// si por data sucia existieran múltiples invites/activos, al aceptar uno debe quedar 1 activo.
	// (Si tu Accept no revoca otros todavía, este test te va a fallar — y ahí sí lo corregimos).
	repo := newTestRepo()
	svc := NewService(repo)

	now := time.Date(2025, 12, 22, 10, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	// Seed “data sucia”: 2 invites para el mismo (pet, owner, grantee)
	g1 := Grant{
		ID:            "g1",
		PetID:         "pet-1",
		OwnerUserID:   "owner-1",
		GranteeUserID: "delegate-1",
		Scopes:        []Scope{ScopeEventsRead},
		Status:        StatusInvited,
		CreatedAt:     now.Add(-10 * time.Minute),
		UpdatedAt:     now.Add(-10 * time.Minute),
	}
	g2 := Grant{
		ID:            "g2",
		PetID:         "pet-1",
		OwnerUserID:   "owner-1",
		GranteeUserID: "delegate-1",
		Scopes:        []Scope{ScopeEventsRead},
		Status:        StatusInvited,
		CreatedAt:     now.Add(-5 * time.Minute),
		UpdatedAt:     now.Add(-5 * time.Minute),
	}
	_ = repo.Create(context.Background(), g1)
	_ = repo.Create(context.Background(), g2)

	_, err := svc.Accept(context.Background(), "g2", "delegate-1")
	if err != nil {
		t.Fatalf("Accept error: %v", err)
	}

	// Contar activos para (pet-1, delegate-1)
	activeCount := 0
	for _, g := range repo.byID {
		if g.PetID == "pet-1" && g.GranteeUserID == "delegate-1" && g.Status == StatusActive {
			activeCount++
		}
	}
	if activeCount != 1 {
		t.Fatalf("expected exactly 1 active grant, got %d", activeCount)
	}
}
