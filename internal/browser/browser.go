package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

type Options struct {
	Headless     bool
	ChromePath   string        // vazio = deixa o chromedp autodetectar
	NavTimeout   time.Duration // timeout por operação de navegação
	PageLoadWait time.Duration // espera pós-load para render dinâmico
}
type Navigator interface {
	GetHTML(ctx context.Context, url string) (string, error)
	Close()
}

type Session struct {
	allocCancel context.CancelFunc
	ctx         context.Context
	ctxCancel   context.CancelFunc
	opts        Options
	log         *slog.Logger
}

func NewSession(opts Options, log *slog.Logger) *Session {
	if opts.NavTimeout <= 0 {
		opts.NavTimeout = 45 * time.Second
	}

	execOpts := append([]chromedp.ExecAllocatorOption{},
		chromedp.DefaultExecAllocatorOptions[:]...,
	)
	execOpts = append(execOpts,
		chromedp.Flag("headless", opts.Headless),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("deny-permission-prompts", true),
		chromedp.Flag("disable-notifications", true),
		chromedp.Flag("disable-features", "Translate,MediaRouter"),
	)
	if opts.ChromePath != "" {
		execOpts = append(execOpts, chromedp.ExecPath(opts.ChromePath))
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), execOpts...)
	ctx, ctxCancel := chromedp.NewContext(allocCtx)

	return &Session{
		allocCancel: allocCancel,
		ctx:         ctx,
		ctxCancel:   ctxCancel,
		opts:        opts,
		log:         log,
	}
}

func NewRemoteSession(debugURL string, opts Options, log *slog.Logger) (*Session, error) {
	if opts.NavTimeout <= 0 {
		opts.NavTimeout = 45 * time.Second
	}
	wsURL, err := resolveWSURL(debugURL)
	if err != nil {
		return nil, fmt.Errorf("browser: não consegui falar com o Chrome em %s: %w", debugURL, err)
	}

	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), wsURL)
	ctx, ctxCancel := chromedp.NewContext(allocCtx)

	return &Session{
		allocCancel: allocCancel,
		ctx:         ctx,
		ctxCancel:   ctxCancel,
		opts:        opts,
		log:         log,
	}, nil
}

func resolveWSURL(debugURL string) (string, error) {
	if strings.HasPrefix(debugURL, "ws://") || strings.HasPrefix(debugURL, "wss://") {
		return debugURL, nil
	}
	endpoint := strings.TrimRight(debugURL, "/") + "/json/version"
	c := &http.Client{Timeout: 5 * time.Second}
	resp, err := c.Get(endpoint)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var v struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return "", err
	}
	if v.WebSocketDebuggerURL == "" {
		return "", fmt.Errorf("webSocketDebuggerUrl vazio em %s", endpoint)
	}
	return v.WebSocketDebuggerURL, nil
}

func (s *Session) GetHTML(ctx context.Context, url string) (string, error) {
	runCtx, cancel := context.WithTimeout(s.ctx, s.opts.NavTimeout)
	defer cancel()

	go func() {
		select {
		case <-ctx.Done():
			cancel()
		case <-runCtx.Done():
		}
	}()

	var html string
	tasks := chromedp.Tasks{
		chromedp.Navigate(url),
		chromedp.Sleep(s.opts.PageLoadWait),
		chromedp.OuterHTML("html", &html, chromedp.ByQuery),
	}

	if err := chromedp.Run(runCtx, tasks); err != nil {
		return "", fmt.Errorf("browser: falha ao obter HTML de %s: %w", url, err)
	}
	return html, nil
}

func (s *Session) Ctx() context.Context { return s.ctx }

func (s *Session) Run(actions ...chromedp.Action) error {
	ctx, cancel := context.WithTimeout(s.ctx, s.opts.NavTimeout)
	defer cancel()
	return chromedp.Run(ctx, actions...)
}

func (s *Session) Close() {
	if s.ctxCancel != nil {
		s.ctxCancel()
	}
	if s.allocCancel != nil {
		s.allocCancel()
	}
}

var _ Navigator = (*Session)(nil)
