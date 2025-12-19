package auth

import "context"

// AuthVerifier verifica un token y devuelve claims o error.
type AuthVerifier interface {
	Verify(ctx context.Context, token string) (Claims, error)
}
