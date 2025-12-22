package events

import "time"

// Actor representa quién originó un evento (owner, delegado u otro sistema).
type Actor struct {
	Type ActorType
	ID   string
}

// PetEvent representa un evento clínico o de historial asociado a una mascota.
type PetEvent struct {
	ID    string
	PetID string

	Type EventType

	OccurredAt time.Time
	RecordedAt time.Time

	Title string
	Notes string

	Actor      Actor
	Source     Source
	Visibility Visibility
	Status     EventStatus
}
