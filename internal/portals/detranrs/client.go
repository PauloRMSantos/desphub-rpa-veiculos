package detranrs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/paulorosantos/desphub-rpa/internal/model"
)

var ErrSessionExpired = errors.New("detranrs: session invalid/expired")

const (
	DefaultBaseURL = "https://pcsdetran.procergs.com.br"
	spaOrigin  = "https://pcsdetran.rs.gov.br"
	spaReferer = "https://pcsdetran.rs.gov.br/"
)

type Auth struct {
	Bearer string // Authorization: Bearer <token>
	UserID string 
}

type Client struct {
	http    *http.Client
	baseURL string
	auth    Auth
	ua      string
}

func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		http:    &http.Client{Timeout: 30 * time.Second},
		baseURL: baseURL,
		ua:      "Mozilla/5.0 (compatible; DespHubRPA/1.0)",
	}
}

func (c *Client) SetAuth(a Auth) { c.auth = a }

func (c *Client) SetUserAgent(ua string) {
	if ua != "" {
		c.ua = ua
	}
}

func (c *Client) Name() string { return "DETRAN-RS" }

func (c *Client) Query(ctx context.Context, plate, renavam string) (*model.QueryResponse, error) {
	raw, err := c.queryVehicle(ctx, plate, renavam)
	if err != nil {
		return nil, err
	}
	return ParseVehicle(raw)
}

const maxAttempts = 3

func (c *Client) queryVehicle(ctx context.Context, plate, renavam string) ([]byte, error) {
	if c.auth.Bearer == "" || c.auth.UserID == "" {
		return nil, fmt.Errorf("detranrs: missing credentials (Bearer/X-User-Id) — log in first")
	}

	endpoint := fmt.Sprintf("%s/pcsdetran/rest/veiculos/%s/", c.baseURL, url.PathEscape(plate))
	q := url.Values{}
	q.Set("renavam", renavam)
	q.Set("contabiliza", "false")
	endpoint += "?" + q.Encode()

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		body, status, err := c.doGet(ctx, endpoint)
		switch {
		case err != nil:
			lastErr = err
		case status == http.StatusUnauthorized || status == http.StatusForbidden:
			return nil, fmt.Errorf("%w (HTTP %d)", ErrSessionExpired, status)
		case status >= 500:
			lastErr = fmt.Errorf("detranrs: API returned HTTP %d", status)
		case status != http.StatusOK:
			return nil, fmt.Errorf("detranrs: API returned HTTP %d", status)
		default:
			return body, nil
		}

		if attempt < maxAttempts {
			if err := waitBackoff(ctx, attempt); err != nil {
				return nil, err
			}
		}
	}
	return nil, fmt.Errorf("detranrs: failed after %d attempts: %w", maxAttempts, lastErr)
}

func (c *Client) doGet(ctx context.Context, endpoint string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Authorization", "Bearer "+c.auth.Bearer)
	req.Header.Set("X-User-Id", c.auth.UserID)
	req.Header.Set("Origin", spaOrigin)
	req.Header.Set("Referer", spaReferer)
	req.Header.Set("User-Agent", c.ua)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("detranrs: vehicle request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("detranrs: reading body: %w", err)
	}
	return body, resp.StatusCode, nil
}

func waitBackoff(ctx context.Context, attempt int) error {
	d := time.Duration(200*(1<<(attempt-1))) * time.Millisecond
	select {
	case <-time.After(d):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
