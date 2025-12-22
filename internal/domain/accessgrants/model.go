package accessgrants

import "time"

// Scope define un permiso granular que puede otorgarse a un delegado sobre una mascota.
type Scope string

const (
	// ScopePetRead permite leer el perfil de la mascota.
	ScopePetRead Scope = "pet:read"
	// ScopePetEditProfile permite editar el perfil de la mascota.
	ScopePetEditProfile Scope = "pet:edit_profile"
	// ScopeEventsRead permite leer los eventos clínicos de la mascota.
	ScopeEventsRead Scope = "events:read"
	// ScopeEventsCreate permite crear nuevos eventos clínicos.
	ScopeEventsCreate Scope = "events:create"
	// ScopeEventsVoid permite anular (void) eventos clínicos existentes.
	ScopeEventsVoid Scope = "events:void"
	// ScopeAttachmentsAdd permite adjuntar archivos u otros recursos al historial.
	ScopeAttachmentsAdd Scope = "attachments:add"
)

// Status representa el estado de un grant de acceso delegado.
type Status string

const (
	// StatusInvited indica que el grant fue invitado pero aún no aceptado.
	StatusInvited Status = "invited"
	// StatusActive indica que el grant está aceptado y vigente.
	StatusActive Status = "active"
	// StatusRevoked indica que el grant fue revocado.
	StatusRevoked Status = "revoked"
)

// Grant representa una delegación de acceso de un owner hacia un usuario delegado sobre una mascota.
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
