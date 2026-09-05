package detranrs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

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
		p.log.Info("sessão expirada; tentando refresh silencioso")
		if err := p.refreshSilencioso(ctx); err != nil {
			return nil, err
		}
		resp, err = p.client.Consultar(ctx, placa, renavam)
	}
	return resp, err
}

func (p *SessionPortal) Reconectar(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.authed = false
	return p.loginComTimeout(ctx, LoginManualTimeout)
}

func (p *SessionPortal) garantirLogin(ctx context.Context, forcar bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.authed && !forcar {
		return nil
	}
	return p.loginComTimeout(ctx, LoginManualTimeout)
}

func (p *SessionPortal) refreshSilencioso(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.loginComTimeout(ctx, LoginSilentTimeout); err != nil {
		p.log.Warn("refresh silencioso falhou; requer reconexão manual", "erro", err.Error())
		return fmt.Errorf("%w (detalhe: %v)", model.ErrReconexaoNecessaria, err)
	}
	return nil
}

func (p *SessionPortal) loginComTimeout(ctx context.Context, timeout time.Duration) error {
	auth, err := Login(ctx, p.sess, p.log, timeout)
	if err != nil {
		p.authed = false
		return err
	}
	p.client.SetAuth(auth)
	p.authed = true
	return nil
}
