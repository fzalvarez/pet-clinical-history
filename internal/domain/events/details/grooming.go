package details

type GroomingKind string

const (
	GroomingKindBath GroomingKind = "bath"
)

type Grooming struct {
	ID      string
	EventID string

	Kind  GroomingKind
	Notes string
}
