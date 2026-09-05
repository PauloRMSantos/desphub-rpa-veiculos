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
		t.Skip("defina RUN_LOGIN_PROBE=1 e um Chrome com --remote-debugging-port aberto")
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
		t.Fatalf("conectar ao Chrome (%s) falhou: %v — abriu o Chrome com --remote-debugging-port=9222?", debugURL, err)
	}
	defer sess.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()

	auth, err := Login(ctx, sess, log, LoginManualTimeout)
	if err != nil {
		t.Fatalf("Login falhou: %v", err)
	}
	if auth.Bearer == "" {
		t.Fatal("Bearer vazio — não capturado")
	}
	t.Logf("OK captura → Bearer=%s… (len=%d)  X-User-Id=%q",
		mascarar(auth.Bearer), len(auth.Bearer), auth.UserID)

	if placa, renavam := os.Getenv("TEST_PLACA"), os.Getenv("TEST_RENAVAM"); placa != "" && renavam != "" {
		c := NewClient(DefaultBaseURL)
		c.SetAuth(auth)
		resp, err := c.Consultar(ctx, placa, renavam)
		if err != nil {
			t.Fatalf("Consultar falhou: %v", err)
		}
		t.Logf("veículo: %s (%d) — situação %q",
			resp.Veiculo.MarcaModelo, resp.Veiculo.AnoModelo, resp.Veiculo.SituacaoRenavam)
	}
}

func mascarar(s string) string {
	if len(s) <= 8 {
		return "****"
	}
	return s[:8]
}
