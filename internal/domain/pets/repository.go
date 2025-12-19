package pets

import "context"

type Repository interface {
	Create(ctx context.Context, p Pet) error
	GetByID(ctx context.Context, id string) (Pet, error)
	ListByOwner(ctx context.Context, ownerUserID string) ([]Pet, error)
}
