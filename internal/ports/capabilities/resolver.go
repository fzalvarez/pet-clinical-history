package capabilities

import "context"

type CapabilitiesResolver interface {
	HasFeature(ctx context.Context, in CapabilityCheck) (bool, error)
}
