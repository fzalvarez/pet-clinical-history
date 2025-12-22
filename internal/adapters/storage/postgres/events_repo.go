package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"pet-clinical-history/internal/domain/events"
)

type EventsRepo struct {
	db *sql.DB
}

func NewEventsRepo(db *sql.DB) *EventsRepo {
	return &EventsRepo{db: db}
}

func (r *EventsRepo) Create(ctx context.Context, e events.PetEvent) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO pet_events (
			id, pet_id,
			type, occurred_at, recorded_at,
			title, notes,
			actor_type, actor_id,
			source, visibility,
			status
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	`,
		e.ID,
		e.PetID,
		string(e.Type),
		e.OccurredAt,
		e.RecordedAt,
		e.Title,
		e.Notes,
		string(e.Actor.Type),
		e.Actor.ID,
		string(e.Source),
		string(e.Visibility),
		string(e.Status),
	)
	return err
}

func (r *EventsRepo) GetByID(ctx context.Context, id string) (events.PetEvent, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return events.PetEvent{}, ErrNotFound
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT
			id, pet_id,
			type, occurred_at, recorded_at,
			title, notes,
			actor_type, actor_id,
			source, visibility,
			status
		FROM pet_events
		WHERE id = $1
	`, id)

	var e events.PetEvent
	var typ, actorType, source, vis, status string
	if err := row.Scan(
		&e.ID,
		&e.PetID,
		&typ,
		&e.OccurredAt,
		&e.RecordedAt,
		&e.Title,
		&e.Notes,
		&actorType,
		&e.Actor.ID,
		&source,
		&vis,
		&status,
	); err != nil {
		if err == sql.ErrNoRows {
			return events.PetEvent{}, ErrNotFound
		}
		return events.PetEvent{}, err
	}

	e.Type = events.EventType(typ)
	e.Actor.Type = events.ActorType(actorType)
	e.Source = events.Source(source)
	e.Visibility = events.Visibility(vis)
	e.Status = events.EventStatus(status)

	return e, nil
}

func (r *EventsRepo) ListByPet(ctx context.Context, petID string, filter events.ListFilter) ([]events.PetEvent, error) {
	petID = strings.TrimSpace(petID)
	if petID == "" {
		return nil, nil
	}

	// Base query
	sb := strings.Builder{}
	sb.WriteString(`
		SELECT
			id, pet_id,
			type, occurred_at, recorded_at,
			title, notes,
			actor_type, actor_id,
			source, visibility,
			status
		FROM pet_events
		WHERE pet_id = $1
	`)

	args := []any{petID}
	argN := 2

	// types filter
	if len(filter.Types) > 0 {
		placeholders := make([]string, 0, len(filter.Types))
		for _, t := range filter.Types {
			placeholders = append(placeholders, fmt.Sprintf("$%d", argN))
			args = append(args, string(t))
			argN++
		}
		sb.WriteString(" AND type IN (" + strings.Join(placeholders, ",") + ")")
	}

	// from/to
	if filter.From != nil {
		sb.WriteString(fmt.Sprintf(" AND occurred_at >= $%d", argN))
		args = append(args, *filter.From)
		argN++
	}
	if filter.To != nil {
		sb.WriteString(fmt.Sprintf(" AND occurred_at <= $%d", argN))
		args = append(args, *filter.To)
		argN++
	}

	// q: búsqueda simple en title + notes
	if strings.TrimSpace(filter.Query) != "" {
		sb.WriteString(fmt.Sprintf(" AND (title ILIKE $%d OR notes ILIKE $%d)", argN, argN))
		args = append(args, "%"+strings.TrimSpace(filter.Query)+"%")
		argN++
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	sb.WriteString(" ORDER BY occurred_at DESC")
	sb.WriteString(fmt.Sprintf(" LIMIT $%d", argN))
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]events.PetEvent, 0)
	for rows.Next() {
		var e events.PetEvent
		var typ, actorType, source, vis, status string

		if err := rows.Scan(
			&e.ID,
			&e.PetID,
			&typ,
			&e.OccurredAt,
			&e.RecordedAt,
			&e.Title,
			&e.Notes,
			&actorType,
			&e.Actor.ID,
			&source,
			&vis,
			&status,
		); err != nil {
			return nil, err
		}

		e.Type = events.EventType(typ)
		e.Actor.Type = events.ActorType(actorType)
		e.Source = events.Source(source)
		e.Visibility = events.Visibility(vis)
		e.Status = events.EventStatus(status)

		out = append(out, e)
	}

	return out, rows.Err()
}

func (r *EventsRepo) Void(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return ErrNotFound
	}

	res, err := r.db.ExecContext(ctx, `
		UPDATE pet_events
		SET status = 'voided'
		WHERE id = $1
	`, id)
	if err != nil {
		return err
	}

	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
