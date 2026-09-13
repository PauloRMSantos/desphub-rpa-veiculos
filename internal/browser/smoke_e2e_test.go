//go:build e2e

// chromedp integration smoke test. Runs only with `-tags e2e` and requires an
// installed Chrome/Chromium. Not part of the default CI.
//
//	go test -tags e2e ./internal/browser -run TestSmokeOpenPortal -v
package browser

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"
)

func TestSmokeOpenPortal(t *testing.T) {
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
		t.Fatalf("GetHTML failed: %v", err)
	}
	if !strings.Contains(strings.ToLower(html), "<html") {
		t.Fatalf("rendered HTML looks empty/invalid (len=%d)", len(html))
	}
	t.Logf("OK: %s rendered %d bytes of HTML", url, len(html))
}
