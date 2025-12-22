package plansfeatures

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var (
	ErrPlansNotConfigured = errors.New("plans-features client not configured")
	ErrPlansUnauthorized  = errors.New("plans-features unauthorized")
	ErrPlansUpstream      = errors.New("plans-features upstream error")
)

type Config struct {
	BaseURL string
	APIKey  string

	APIKeyHeader string
	Timeout      time.Duration
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

// CapabilitiesResponse es deliberadamente simple.
// Cuando plans-features esté listo, se adapta al contrato real.
type CapabilitiesResponse struct {
	// Ejemplo: {"pet:attachments:add": true, "events:void": false}
	Capabilities map[string]bool `json:"capabilities"`
}

// GetCapabilities trae capabilities para un usuario.
// ⚠️ Endpoint placeholder. Ajustar cuando exista contrato real.
func (c *Client) GetCapabilities(ctx context.Context, userID string) (CapabilitiesResponse, error) {
	if !c.IsConfigured() {
		return CapabilitiesResponse{}, ErrPlansNotConfigured
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return CapabilitiesResponse{}, errors.New("userID required")
	}

	// TODO(plans-features): ajustar path según contrato real.
	// Una opción típica: GET /v1/capabilities?user_id=...
	url := fmt.Sprintf("%s/v1/capabilities?user_id=%s", c.baseURL, userID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return CapabilitiesResponse{}, fmt.Errorf("%w: %v", ErrPlansUpstream, err)
	}
	req.Header.Set(c.apiKeyHeader, c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return CapabilitiesResponse{}, fmt.Errorf("%w: %v", ErrPlansUpstream, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// ok
	case http.StatusUnauthorized, http.StatusForbidden:
		return CapabilitiesResponse{}, ErrPlansUnauthorized
	default:
		return CapabilitiesResponse{}, fmt.Errorf("%w: status=%d", ErrPlansUpstream, resp.StatusCode)
	}

	var out CapabilitiesResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return CapabilitiesResponse{}, fmt.Errorf("%w: invalid json: %v", ErrPlansUpstream, err)
	}
	if out.Capabilities == nil {
		out.Capabilities = map[string]bool{}
	}
	return out, nil
}
