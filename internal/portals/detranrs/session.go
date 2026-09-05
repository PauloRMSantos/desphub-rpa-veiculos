package detranrs

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/paulorosantos/desphub-rpa/internal/browser"
	"github.com/paulorosantos/desphub-rpa/internal/model"
)

type SessionPortal struct {
	sess   *browser.Session
	client *Client
	log    *slog.Logger

	mu     sync.Mutex
	authed bool
}


func NewSessionPortal(sess *browser.Session, baseURL string, log *slog.Logger) *SessionPortal {
	return &SessionPortal{
		sess:   sess,
		client: NewClient(baseURL),
		log:    log,
	}
}

func (p *SessionPortal) Nome() string { return "DETRAN-RS" }

func (p *SessionPortal) Consultar(ctx context.Context, placa, renavam string) (*model.ConsultaResponse, error) {
	if err := p.garantirLogin(ctx, false); err != nil {
		return nil, err
	}

	resp, err := p.client.Consultar(ctx, placa, renavam)
	if errors.Is(err, ErrSessaoExpirada) {
		p.log.Info("sessão expirada; refazendo login")
		if err := p.garantirLogin(ctx, true); err != nil {
			return nil, err
		}
		resp, err = p.client.Consultar(ctx, placa, renavam)
	}
	return resp, err
}

func (p *SessionPortal) garantirLogin(ctx context.Context, forcar bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.authed && !forcar {
		return nil
	}
	auth, err := Login(ctx, p.sess, p.log)
	if err != nil {
		p.authed = false
		return err
	}
	p.client.SetAuth(auth)
	p.authed = true
	return nil
}
