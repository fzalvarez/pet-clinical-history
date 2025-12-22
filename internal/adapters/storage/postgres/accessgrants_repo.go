package postgres

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"pet-clinical-history/internal/domain/accessgrants"
)

type AccessGrantsRepo struct {
	db *sql.DB
}

func NewAccessGrantsRepo(db *sql.DB) *AccessGrantsRepo {
	return &AccessGrantsRepo{db: db}
}

func (r *AccessGrantsRepo) Create(ctx context.Context, g accessgrants.Grant) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO access_grants (
			id, pet_id, owner_user_id, grantee_user_id,
			scopes, status,
			created_at, updated_at, revoked_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`,
		g.ID,
		g.PetID,
		g.OwnerUserID,
		g.GranteeUserID,
		scopesToTextArray(g.Scopes),
		string(g.Status),
		g.CreatedAt,
		g.UpdatedAt,
		toNullTime(g.RevokedAt),
	)
	return err
}

func (r *AccessGrantsRepo) Update(ctx context.Context, g accessgrants.Grant) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE access_grants
		SET
			scopes = $2,
			status = $3,
			updated_at = $4,
			revoked_at = $5
		WHERE id = $1
	`,
		g.ID,
		scopesToTextArray(g.Scopes),
		string(g.Status),
		g.UpdatedAt,
		toNullTime(g.RevokedAt),
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

func (r *AccessGrantsRepo) GetByID(ctx context.Context, id string) (accessgrants.Grant, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return accessgrants.Grant{}, ErrNotFound
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT
			id, pet_id, owner_user_id, grantee_user_id,
			scopes, status,
			created_at, updated_at, revoked_at
		FROM access_grants
		WHERE id = $1
	`, id)

	var g accessgrants.Grant
	var status string
	var scopes []string
	var revokedAt sql.NullTime

	if err := row.Scan(
		&g.ID,
		&g.PetID,
		&g.OwnerUserID,
		&g.GranteeUserID,
		&scopes,
		&status,
		&g.CreatedAt,
		&g.UpdatedAt,
		&revokedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return accessgrants.Grant{}, ErrNotFound
		}
		return accessgrants.Grant{}, err
	}

	g.Status = accessgrants.Status(status)
	g.Scopes = textArrayToScopes(scopes)
	if revokedAt.Valid {
		t := revokedAt.Time
		g.RevokedAt = &t
	}

	return g, nil
}

func (r *AccessGrantsRepo) ListByPet(ctx context.Context, petID string) ([]accessgrants.Grant, error) {
	petID = strings.TrimSpace(petID)
	if petID == "" {
		return nil, nil
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT
			id, pet_id, owner_user_id, grantee_user_id,
			scopes, status,
			created_at, updated_at, revoked_at
		FROM access_grants
		WHERE pet_id = $1
		ORDER BY created_at ASC
	`, petID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]accessgrants.Grant, 0)
	for rows.Next() {
		var g accessgrants.Grant
		var status string
		var scopes []string
		var revokedAt sql.NullTime

		if err := rows.Scan(
			&g.ID,
			&g.PetID,
			&g.OwnerUserID,
			&g.GranteeUserID,
			&scopes,
			&status,
			&g.CreatedAt,
			&g.UpdatedAt,
			&revokedAt,
		); err != nil {
			return nil, err
		}

		g.Status = accessgrants.Status(status)
		g.Scopes = textArrayToScopes(scopes)
		if revokedAt.Valid {
			t := revokedAt.Time
			g.RevokedAt = &t
		}

		out = append(out, g)
	}

	return out, rows.Err()
}

func (r *AccessGrantsRepo) GetActiveGrant(ctx context.Context, petID, granteeUserID string) (accessgrants.Grant, error) {
	petID = strings.TrimSpace(petID)
	granteeUserID = strings.TrimSpace(granteeUserID)
	if petID == "" || granteeUserID == "" {
		return accessgrants.Grant{}, ErrNotFound
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT
			id, pet_id, owner_user_id, grantee_user_id,
			scopes, status,
			created_at, updated_at, revoked_at
		FROM access_grants
		WHERE pet_id = $1
		  AND grantee_user_id = $2
		  AND status = 'active'
		ORDER BY updated_at DESC
		LIMIT 1
	`, petID, granteeUserID)

	var g accessgrants.Grant
	var status string
	var scopes []string
	var revokedAt sql.NullTime

	if err := row.Scan(
		&g.ID,
		&g.PetID,
		&g.OwnerUserID,
		&g.GranteeUserID,
		&scopes,
		&status,
		&g.CreatedAt,
		&g.UpdatedAt,
		&revokedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return accessgrants.Grant{}, ErrNotFound
		}
		return accessgrants.Grant{}, err
	}

	g.Status = accessgrants.Status(status)
	g.Scopes = textArrayToScopes(scopes)
	if revokedAt.Valid {
		t := revokedAt.Time
		g.RevokedAt = &t
	}

	return g, nil
}

func (r *AccessGrantsRepo) ListByGrantee(ctx context.Context, granteeUserID string) ([]accessgrants.Grant, error) {
	granteeUserID = strings.TrimSpace(granteeUserID)
	if granteeUserID == "" {
		return nil, nil
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT
			id, pet_id, owner_user_id, grantee_user_id,
			scopes, status,
			created_at, updated_at, revoked_at
		FROM access_grants
		WHERE grantee_user_id = $1
		ORDER BY updated_at DESC
	`, granteeUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]accessgrants.Grant, 0)
	for rows.Next() {
		var g accessgrants.Grant
		var status string
		var scopes []string
		var revokedAt sql.NullTime

		if err := rows.Scan(
			&g.ID,
			&g.PetID,
			&g.OwnerUserID,
			&g.GranteeUserID,
			&scopes,
			&status,
			&g.CreatedAt,
			&g.UpdatedAt,
			&revokedAt,
		); err != nil {
			return nil, err
		}

		g.Status = accessgrants.Status(status)
		g.Scopes = textArrayToScopes(scopes)
		if revokedAt.Valid {
			t := revokedAt.Time
			g.RevokedAt = &t
		}

		out = append(out, g)
	}

	return out, rows.Err()
}

// helpers
func scopesToTextArray(in []accessgrants.Scope) []string {
	if len(in) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(in))
	for _, s := range in {
		out = append(out, string(s))
	}
	return out
}

func textArrayToScopes(in []string) []accessgrants.Scope {
	if len(in) == 0 {
		return []accessgrants.Scope{}
	}
	out := make([]accessgrants.Scope, 0, len(in))
	for _, s := range in {
		out = append(out, accessgrants.Scope(s))
	}
	return out
}

func toNullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{Valid: false}
	}
	return sql.NullTime{Time: *t, Valid: true}
}
