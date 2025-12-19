package details

import "time"

type Medication struct {
	ID      string
	EventID string

	Name string

	Dosage   string // "2"
	DoseUnit string // "ml", "mg", etc.

	Frequency string // texto por ahora: "cada 12h"

	StartDate time.Time
	EndDate   *time.Time

	Notes string
}
