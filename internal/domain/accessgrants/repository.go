package accessgrants

import "context"

type Repository interface {
	Create(ctx context.Context, g Grant) error
	Update(ctx context.Context, g Grant) error
	GetByID(ctx context.Context, id string) (Grant, error)
	ListByPet(ctx context.Context, petID string) ([]Grant, error)
	GetActiveGrant(ctx context.Context, petID, granteeUserID string) (Grant, error)
}
