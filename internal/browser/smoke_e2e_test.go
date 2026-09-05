//go:build e2e

// Smoke test de integração do chromedp. Roda somente com `-tags e2e` e exige
// um Chrome/Chromium instalado. Não roda no CI padrão.
//
//	go test -tags e2e ./internal/browser -run TestSmokeAbrePortal -v
package browser

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"
)

func TestSmokeAbrePortal(t *testing.T) {
	url := os.Getenv("SMOKE_URL")
	if url == "" {
		url = "https://sso.acesso.gov.br/login"
	}

	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	sess := NewSession(Options{
		Headless:     os.Getenv("HEADLESS") != "false",
		NavTimeout:   45 * time.Second,
		PageLoadWait: 2 * time.Second,
	}, log)
	defer sess.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	html, err := sess.GetHTML(ctx, url)
	if err != nil {
		t.Fatalf("GetHTML falhou: %v", err)
	}
	if !strings.Contains(strings.ToLower(html), "<html") {
		t.Fatalf("HTML renderizado parece vazio/ inválido (len=%d)", len(html))
	}
	t.Logf("OK: %s renderizou %d bytes de HTML", url, len(html))
}
