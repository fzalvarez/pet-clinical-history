package details

import "time"

type PreventiveKind string

const (
	PreventiveKindDeworming     PreventiveKind = "deworming"
	PreventiveKindFleaTreatment PreventiveKind = "flea_treatment"
)

type PreventiveTreatment struct {
	ID      string
	EventID string

	Kind PreventiveKind

	Product string
	Dose    string

	NextDue *time.Time
	Notes   string
}
