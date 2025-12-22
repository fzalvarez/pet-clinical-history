package postgres

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"pet-clinical-history/internal/domain/pets"
)

type PetsRepo struct {
	db *sql.DB
}

func NewPetsRepo(db *sql.DB) *PetsRepo {
	return &PetsRepo{db: db}
}

func (r *PetsRepo) Create(ctx context.Context, p pets.Pet) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO pets (
			id, owner_user_id,
			name, species, breed, sex,
			birth_date, microchip, notes,
			created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
	`,
		p.ID,
		p.OwnerUserID,
		p.Name,
		p.Species,
		p.Breed,
		p.Sex,
		toNullDate(p.BirthDate),
		p.Microchip,
		p.Notes,
		p.CreatedAt,
		p.UpdatedAt,
	)
	return err
}

func (r *PetsRepo) Update(ctx context.Context, p pets.Pet) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE pets
		SET
			name = $2,
			species = $3,
			breed = $4,
			sex = $5,
			birth_date = $6,
			microchip = $7,
			notes = $8,
			updated_at = $9
		WHERE id = $1
	`,
		p.ID,
		p.Name,
		p.Species,
		p.Breed,
		p.Sex,
		toNullDate(p.BirthDate),
		p.Microchip,
		p.Notes,
		p.UpdatedAt,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PetsRepo) GetByID(ctx context.Context, id string) (pets.Pet, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return pets.Pet{}, ErrNotFound
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT
			id, owner_user_id,
			name, species, breed, sex,
			birth_date, microchip, notes,
			created_at, updated_at
		FROM pets
		WHERE id = $1
	`, id)

	var p pets.Pet
	var bd sql.NullTime
	if err := row.Scan(
		&p.ID,
		&p.OwnerUserID,
		&p.Name,
		&p.Species,
		&p.Breed,
		&p.Sex,
		&bd,
		&p.Microchip,
		&p.Notes,
		&p.CreatedAt,
		&p.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return pets.Pet{}, ErrNotFound
		}
		return pets.Pet{}, err
	}

	if bd.Valid {
		t := bd.Time
		// ojo: birth_date es date, pgx lo puede mapear a time.Time midnight UTC
		p.BirthDate = &t
	} else {
		p.BirthDate = nil
	}

	return p, nil
}

func (r *PetsRepo) ListByOwner(ctx context.Context, ownerUserID string) ([]pets.Pet, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" {
		return nil, nil
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT
			id, owner_user_id,
			name, species, breed, sex,
			birth_date, microchip, notes,
			created_at, updated_at
		FROM pets
		WHERE owner_user_id = $1
		ORDER BY created_at ASC
	`, ownerUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]pets.Pet, 0)
	for rows.Next() {
		var p pets.Pet
		var bd sql.NullTime
		if err := rows.Scan(
			&p.ID,
			&p.OwnerUserID,
			&p.Name,
			&p.Species,
			&p.Breed,
			&p.Sex,
			&bd,
			&p.Microchip,
			&p.Notes,
			&p.CreatedAt,
			&p.UpdatedAt,
		); err != nil {
			return nil, err
		}

		if bd.Valid {
			t := bd.Time
			p.BirthDate = &t
		}

		out = append(out, p)
	}

	return out, rows.Err()
}

// birth_date es DATE, lo pasamos como NullTime para simplificar
func toNullDate(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{Valid: false}
	}
	return sql.NullTime{Time: *t, Valid: true}
}
