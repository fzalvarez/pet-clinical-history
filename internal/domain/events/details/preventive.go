package details

import "time"

// PreventiveKind representa el tipo de tratamiento preventivo registrado en un evento.
type PreventiveKind string

const (
	// PreventiveKindDeworming indica un tratamiento de desparasitación.
	PreventiveKindDeworming PreventiveKind = "deworming"
	// PreventiveKindFleaTreatment indica un tratamiento contra pulgas.
	PreventiveKindFleaTreatment PreventiveKind = "flea_treatment"
)

// PreventiveTreatment modela el detalle de un tratamiento preventivo asociado a un evento.
type PreventiveTreatment struct {
	ID      string
	EventID string

	Kind PreventiveKind

	Product string
	Dose    string

	NextDue *time.Time
	Notes   string
}
