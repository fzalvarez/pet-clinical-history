package odin

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"pet-clinical-history/internal/ports/auth"
)

var (
	ErrTokenEmpty = errors.New("token is empty")
)

// Verifier implementa auth.AuthVerifier usando Odin.
// No se integra automáticamente; queda listo para que lo instancien desde main/router.
type Verifier struct {
	client *Client
}

func NewVerifier(client *Client) *Verifier {
	return &Verifier{client: client}
}

func (v *Verifier) Verify(ctx context.Context, token string) (auth.Claims, error) {
	if v == nil || v.client == nil {
		return auth.Claims{}, ErrOdinNotConfigured
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return auth.Claims{}, ErrTokenEmpty
	}

	claims, err := v.client.VerifyToken(ctx, token)
	if err != nil {
		// Normalizamos un poco, pero sin “inventar” semantics.
		// El middleware actual ya decide si corta o no.
		return auth.Claims{}, fmt.Errorf("odin verify failed: %w", err)
	}

	claims.UserID = strings.TrimSpace(claims.UserID)
	if claims.UserID == "" {
		return auth.Claims{}, errors.New("odin claims missing user id")
	}

	return claims, nil
}
