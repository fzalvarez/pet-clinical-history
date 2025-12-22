package pets

import "time"

// Pet representa el perfil básico de una mascota registrada en el sistema.
type Pet struct {
	ID          string
	OwnerUserID string

	Name    string
	Species string // dog, cat, etc. (texto por ahora)
	Breed   string
	Sex     string // male/female/unknown (texto por ahora)

	BirthDate *time.Time
	Microchip string

	Notes string

	CreatedAt time.Time
	UpdatedAt time.Time
}
