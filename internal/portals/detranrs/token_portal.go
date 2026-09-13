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

	mu       sync.RWMutex
	authed   bool
	expiraEm time.Time 
}

func NewTokenPortal(baseURL string) *TokenPortal {
	return &TokenPortal{client: NewClient(baseURL)}
}

func (p *TokenPortal) Nome() string { return "DETRAN-RS" }

func (p *TokenPortal) SetToken(bearer, userID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.client.SetAuth(Auth{Bearer: bearer, UserID: userID})
	p.authed = bearer != "" && userID != ""
	p.expiraEm = jwtExpiracao(bearer)
}

func (p *TokenPortal) Status() (authed bool, expiraEm time.Time) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.authed, p.expiraEm
}

func (p *TokenPortal) Consultar(ctx context.Context, placa, renavam string) (*model.ConsultaResponse, error) {
	p.mu.RLock()
	authed := p.authed
	p.mu.RUnlock()
	if !authed {
		return nil, model.ErrReconexaoNecessaria
	}

	resp, err := p.client.Consultar(ctx, placa, renavam)
	if errors.Is(err, ErrSessaoExpirada) {
		p.mu.Lock()
		p.authed = false
		p.mu.Unlock()
		return nil, model.ErrReconexaoNecessaria
	}
	return resp, err
}

func jwtExpiracao(token string) time.Time {
	partes := strings.Split(token, ".")
	if len(partes) < 2 {
		return time.Time{}
	}
	payload, err := base64.RawURLEncoding.DecodeString(partes[1])
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
