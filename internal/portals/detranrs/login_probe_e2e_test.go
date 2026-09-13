//go:build e2e

// Manual gov.br -> DETRAN-RS login probe. Not part of CI. Connects to the Chrome
// YOU opened with remote debugging (so gov.br's captcha is not invalidated by
// bot detection) and confirms the Bearer/X-User-Id capture.
//
// 1) Open a dedicated Chrome with remote debugging (own profile, persists login):
//
//	"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
//	  --remote-debugging-port=9222 --user-data-dir="$HOME/.desphub-chrome" \
//	  https://pcsdetran.rs.gov.br/consulta-veiculo?contabiliza=true
//
// 2) Run the probe (it opens a tab in that Chrome; log in to gov.br):
//
//	RUN_LOGIN_PROBE=1 TEST_PLATE=IDX1756 TEST_RENAVAM=00561040575 \
//	  go test -tags e2e ./internal/portals/detranrs -run TestLoginProbe -v -timeout 360s
//
// The output masks the token — it never prints the full value.
package detranrs

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/paulorosantos/desphub-rpa/internal/browser"
)

func TestLoginProbe(t *testing.T) {
	if os.Getenv("RUN_LOGIN_PROBE") == "" {
		t.Skip("set RUN_LOGIN_PROBE=1 and open a Chrome with --remote-debugging-port")
	}
	debugURL := os.Getenv("CHROME_DEBUG_URL")
	if debugURL == "" {
		debugURL = "http://localhost:9222"
	}

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	sess, err := browser.NewRemoteSession(debugURL, browser.Options{
		NavTimeout:   90 * time.Second,
		PageLoadWait: 2 * time.Second,
	}, log)
	if err != nil {
		t.Fatalf("connecting to Chrome (%s) failed: %v — did you open Chrome with --remote-debugging-port=9222?", debugURL, err)
	}
	defer sess.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()

	auth, err := Login(ctx, sess, log, LoginManualTimeout)
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if auth.Bearer == "" {
		t.Fatal("empty Bearer — not captured")
	}
	t.Logf("OK capture -> Bearer=%s… (len=%d)  X-User-Id=%q", mask(auth.Bearer), len(auth.Bearer), auth.UserID)

	// Optional end-to-end proof: query a vehicle if plate/renavam are provided.
	if plate, renavam := os.Getenv("TEST_PLATE"), os.Getenv("TEST_RENAVAM"); plate != "" && renavam != "" {
		c := NewClient(DefaultBaseURL)
		c.SetAuth(auth)
		resp, err := c.Query(ctx, plate, renavam)
		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}
		t.Logf("vehicle: %s (%d) — status %q", resp.Vehicle.MakeModel, resp.Vehicle.ModelYear, resp.Vehicle.RenavamStatus)
	}
}

func mask(s string) string {
	if len(s) <= 8 {
		return "****"
	}
	return s[:8]
}
