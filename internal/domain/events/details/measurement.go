package details

type MeasurementKind string

const (
	MeasurementKindWeight MeasurementKind = "weight"
)

type Measurement struct {
	ID      string
	EventID string

	Kind  MeasurementKind
	Value float64
	Unit  string // "kg", "lb"
}
