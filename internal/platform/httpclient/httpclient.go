package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultTimeout = 10 * time.Second
)

// Client envuelve *http.Client con helpers comunes para adapters.
type Client struct {
	HTTP    *http.Client
	BaseURL string // opcional; si se define, DoJSON puede recibir paths relativos
}

// New crea un Client con timeout razonable.
func New(timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &Client{
		HTTP: &http.Client{
			Timeout: timeout,
		},
	}
}

// NewWithBaseURL crea un Client con BaseURL + timeout.
func NewWithBaseURL(baseURL string, timeout time.Duration) (*Client, error) {
	c := New(timeout)
	if strings.TrimSpace(baseURL) == "" {
		return c, nil
	}
	_, err := url.ParseRequestURI(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base url: %w", err)
	}
	c.BaseURL = strings.TrimRight(baseURL, "/")
	return c, nil
}

// NewWithTransport permite inyectar un Transport (p.ej. para tests).
func NewWithTransport(timeout time.Duration, tr http.RoundTripper) *Client {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	if tr == nil {
		tr = http.DefaultTransport
	}
	return &Client{
		HTTP: &http.Client{
			Timeout:   timeout,
			Transport: tr,
		},
	}
}

// HTTPError representa una respuesta no-2xx.
type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("http error: status=%d", e.StatusCode)
	}
	return fmt.Sprintf("http error: status=%d body=%s", e.StatusCode, e.Body)
}

// DoJSON hace un request JSON.
// - method: GET/POST/etc
// - pathOrURL: puede ser URL absoluta o path relativo si BaseURL está seteado
// - headers: headers extra (opcional)
// - in: body a enviar (opcional). Si nil => no body.
// - out: donde decodificar JSON (opcional). Si nil => ignora body.
// Retorna error si status no es 2xx.
func (c *Client) DoJSON(
	ctx context.Context,
	method string,
	pathOrURL string,
	headers map[string]string,
	in any,
	out any,
) error {
	if c == nil || c.HTTP == nil {
		return errors.New("httpclient: nil client")
	}

	fullURL, err := c.resolveURL(pathOrURL)
	if err != nil {
		return err
	}

	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("httpclient: marshal json: %w", err)
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return fmt.Errorf("httpclient: new request: %w", err)
	}

	// Defaults
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Extra headers
	for k, v := range headers {
		if strings.TrimSpace(k) == "" {
			continue
		}
		req.Header.Set(k, v)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("httpclient: do request: %w", err)
	}
	defer resp.Body.Close()

	// Leer body (limitado) para errores / decode
	raw, _ := readAtMost(resp.Body, 1<<20) // 1MB max

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &HTTPError{
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(raw)),
		}
	}

	if out == nil {
		return nil
	}
	if len(raw) == 0 {
		return nil
	}

	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("httpclient: unmarshal json: %w", err)
	}

	return nil
}

func (c *Client) resolveURL(pathOrURL string) (string, error) {
	pathOrURL = strings.TrimSpace(pathOrURL)
	if pathOrURL == "" {
		return "", errors.New("httpclient: empty url")
	}

	// Si ya es URL absoluta, úsala tal cual.
	if strings.HasPrefix(pathOrURL, "http://") || strings.HasPrefix(pathOrURL, "https://") {
		return pathOrURL, nil
	}

	// Si no es absoluta, requiere BaseURL.
	if strings.TrimSpace(c.BaseURL) == "" {
		return "", errors.New("httpclient: relative path requires BaseURL")
	}

	if !strings.HasPrefix(pathOrURL, "/") {
		pathOrURL = "/" + pathOrURL
	}
	return c.BaseURL + pathOrURL, nil
}

func readAtMost(r io.Reader, max int64) ([]byte, error) {
	if max <= 0 {
		max = 1 << 20
	}
	lr := io.LimitReader(r, max)
	return io.ReadAll(lr)
}
