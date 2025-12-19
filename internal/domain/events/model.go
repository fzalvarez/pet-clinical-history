package events

import "time"

type Actor struct {
	Type ActorType
	ID   string
}

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
