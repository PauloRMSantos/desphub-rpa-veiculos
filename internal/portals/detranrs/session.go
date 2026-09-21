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
	debugURL string          
	navOpts  browser.Options 
	client   *Client
	log      *slog.Logger

	mu     sync.Mutex
	authed bool
}

func NewSessionPortal(debugURL string, navOpts browser.Options, baseURL string, log *slog.Logger) *SessionPortal {
	return &SessionPortal{
		debugURL: debugURL,
		navOpts:  navOpts,
		client:   NewClient(baseURL),
		log:      log,
	}
}

func (p *SessionPortal) Name() string { return "DETRAN-RS" }

// Query ignores per-request creds: manual mode uses the operator's browser
// session captured into the client. (Per-request credentials are the token
// mode's multi-tenant path.)
func (p *SessionPortal) Query(ctx context.Context, plate, renavam string, _ *model.SessionCredentials) (*model.QueryResponse, error) {
	if err := p.ensureLogin(ctx, false); err != nil {
		return nil, err
	}

	resp, err := p.client.Query(ctx, plate, renavam)
	if errors.Is(err, ErrSessionExpired) {
		p.log.Info("session expired; trying silent refresh")
		if err := p.silentRefresh(ctx); err != nil {
			return nil, err
		}
		resp, err = p.client.Query(ctx, plate, renavam)
	}
	return resp, err
}

func (p *SessionPortal) Reconnect(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.authed = false
	return p.loginWithTimeout(ctx, LoginManualTimeout)
}

func (p *SessionPortal) ensureLogin(ctx context.Context, force bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.authed && !force {
		return nil
	}
	return p.loginWithTimeout(ctx, LoginManualTimeout)
}

func (p *SessionPortal) silentRefresh(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.loginWithTimeout(ctx, LoginSilentTimeout); err != nil {
		p.log.Warn("silent refresh failed; manual reconnect required", "error", err.Error())
		return fmt.Errorf("%w (detail: %v)", model.ErrReconnectRequired, err)
	}
	return nil
}

func (p *SessionPortal) loginWithTimeout(ctx context.Context, timeout time.Duration) error {
	sess, err := browser.NewRemoteSession(p.debugURL, p.navOpts, p.log)
	if err != nil {
		p.authed = false
		return fmt.Errorf("connecting to Chrome (%s): %w", p.debugURL, err)
	}
	defer sess.Close()

	auth, err := Login(ctx, sess, p.log, timeout)
	if err != nil {
		p.authed = false
		return err
	}
	p.client.SetAuth(auth)
	p.authed = true
	return nil
}
