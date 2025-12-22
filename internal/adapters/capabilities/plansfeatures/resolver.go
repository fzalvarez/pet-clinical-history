package plansfeatures

import (
	"context"
	"errors"
	"os"
	"strings"
)

// Resolver es el componente que el motor usaría para decidir capabilities.
// Aún no se integra a handlers; queda como esqueleto para el dev que conecte plans-features.
type Resolver struct {
	client   *Client
	allowAll bool
}

// NewResolver crea un resolver.
// Si ALLOW_ALL_CAPABILITIES=true (env), todo devuelve true (modo dev / fallback).
func NewResolver(client *Client) *Resolver {
	allowAll := strings.EqualFold(strings.TrimSpace(os.Getenv("ALLOW_ALL_CAPABILITIES")), "true")
	return &Resolver{
		client:   client,
		allowAll: allowAll,
	}
}

// Has responde si userID tiene una capability.
// Si allowAll está activo, devuelve true sin llamar a upstream.
func (r *Resolver) Has(ctx context.Context, userID string, capability string) (bool, error) {
	capability = strings.TrimSpace(capability)
	if capability == "" {
		return false, errors.New("capability required")
	}

	if r.allowAll {
		return true, nil
	}

	if r == nil || r.client == nil || !r.client.IsConfigured() {
		// Esqueleto: preferimos fallar explícito en vez de “permitir” sin control.
		return false, ErrPlansNotConfigured
	}

	resp, err := r.client.GetCapabilities(ctx, userID)
	if err != nil {
		return false, err
	}

	return resp.Capabilities[capability], nil
}

// Resolve devuelve el mapa completo de capabilities para userID.
func (r *Resolver) Resolve(ctx context.Context, userID string) (map[string]bool, error) {
	if r.allowAll {
		return map[string]bool{"*": true}, nil
	}
	if r == nil || r.client == nil || !r.client.IsConfigured() {
		return nil, ErrPlansNotConfigured
	}
	resp, err := r.client.GetCapabilities(ctx, userID)
	if err != nil {
		return nil, err
	}
	return resp.Capabilities, nil
}
