package odin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"pet-clinical-history/internal/ports/auth"
)

var (
	ErrOdinNotConfigured = errors.New("odin client not configured")
	ErrOdinUnauthorized  = errors.New("odin unauthorized")
	ErrOdinUpstream      = errors.New("odin upstream error")
)

// Config del cliente Odin.
// BaseURL y APIKey normalmente vendrán de env vars en el servicio que lo instancie.
type Config struct {
	BaseURL string
	APIKey  string

	// Opcional: nombre del header donde se manda la API key.
	// Si está vacío, se usa "X-Api-Key".
	APIKeyHeader string

	// Timeout HTTP (si http.Client es nil, se usa este).
	Timeout time.Duration
}

type Client struct {
	baseURL      string
	apiKey       string
	apiKeyHeader string
	httpClient   *http.Client
}

func NewClient(cfg Config) *Client {
	h := strings.TrimSpace(cfg.APIKeyHeader)
	if h == "" {
		h = "X-Api-Key"
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	return &Client{
		baseURL:      strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"),
		apiKey:       strings.TrimSpace(cfg.APIKey),
		apiKeyHeader: h,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) IsConfigured() bool {
	return c != nil && c.baseURL != "" && c.apiKey != ""
}

// VerifyToken llama a Odin para verificar un token y traer claims.
// ⚠️ Endpoint/payload: es un placeholder estable para el esqueleto.
// Cuando Odin esté listo, reemplazar verifyPath + request/response según contrato real.
func (c *Client) VerifyToken(ctx context.Context, token string) (auth.Claims, error) {
	if !c.IsConfigured() {
		return auth.Claims{}, ErrOdinNotConfigured
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return auth.Claims{}, ErrOdinUnauthorized
	}

	// TODO(odin): ajustar path cuando exista contrato real.
	const verifyPath = "/v1/tokens/verify"

	reqBody := map[string]string{
		"token": token,
	}
	b, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+verifyPath, bytes.NewReader(b))
	if err != nil {
		return auth.Claims{}, fmt.Errorf("%w: %v", ErrOdinUpstream, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(c.apiKeyHeader, c.apiKey)

	// Algunos IAM esperan el token en Authorization, aunque también vaya en body.
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return auth.Claims{}, fmt.Errorf("%w: %v", ErrOdinUpstream, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// ok
	case http.StatusUnauthorized, http.StatusForbidden:
		return auth.Claims{}, ErrOdinUnauthorized
	default:
		return auth.Claims{}, fmt.Errorf("%w: status=%d", ErrOdinUpstream, resp.StatusCode)
	}

	// TODO(odin): ajustar fields reales. Esto es un formato típico.
	var out struct {
		UserID   string `json:"user_id"`
		Email    string `json:"email"`
		TenantID string `json:"tenant_id"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return auth.Claims{}, fmt.Errorf("%w: invalid json: %v", ErrOdinUpstream, err)
	}

	out.UserID = strings.TrimSpace(out.UserID)
	if out.UserID == "" {
		return auth.Claims{}, errors.New("odin response missing user_id")
	}

	return auth.Claims{
		UserID:   out.UserID,
		Email:    strings.TrimSpace(out.Email),
		TenantID: strings.TrimSpace(out.TenantID),
	}, nil
}
