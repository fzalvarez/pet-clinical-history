package accessgrants

import "time"

type Scope string

const (
	ScopePetRead        Scope = "pet:read"
	ScopePetEditProfile Scope = "pet:edit_profile"
	ScopeEventsRead     Scope = "events:read"
	ScopeEventsCreate   Scope = "events:create"
	ScopeEventsVoid     Scope = "events:void"
	ScopeAttachmentsAdd Scope = "attachments:add"
)

type Status string

const (
	StatusInvited Status = "invited"
	StatusActive  Status = "active"
	StatusRevoked Status = "revoked"
)

type Grant struct {
	ID string

	PetID string

	OwnerUserID   string // quien comparte
	GranteeUserID string // delegado

	Scopes []Scope
	Status Status

	CreatedAt time.Time
	UpdatedAt time.Time
	RevokedAt *time.Time
}
