package detranrs

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/paulorosantos/desphub-rpa/internal/model"
)

type TokenPortal struct {
	client *Client

	mu        sync.RWMutex
	authed    bool
	expiresAt time.Time 
}

func NewTokenPortal(baseURL string) *TokenPortal {
	return &TokenPortal{client: NewClient(baseURL)}
}

func (p *TokenPortal) Name() string { return "DETRAN-RS" }

func (p *TokenPortal) SetToken(bearer, userID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.client.SetAuth(Auth{Bearer: bearer, UserID: userID})
	p.authed = bearer != "" && userID != ""
	p.expiresAt = jwtExpiry(bearer)
}

func (p *TokenPortal) Status() (authed bool, expiresAt time.Time) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.authed, p.expiresAt
}

func (p *TokenPortal) Query(ctx context.Context, plate, renavam string) (*model.QueryResponse, error) {
	p.mu.RLock()
	authed := p.authed
	p.mu.RUnlock()
	if !authed {
		return nil, model.ErrReconnectRequired
	}

	resp, err := p.client.Query(ctx, plate, renavam)
	if errors.Is(err, ErrSessionExpired) {
		p.mu.Lock()
		p.authed = false
		p.mu.Unlock()
		return nil, model.ErrReconnectRequired
	}
	return resp, err
}

func jwtExpiry(token string) time.Time {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return time.Time{}
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Exp == 0 {
		return time.Time{}
	}
	return time.Unix(claims.Exp, 0)
}
