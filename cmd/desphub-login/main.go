package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/paulorosantos/desphub-rpa/internal/browser"
	"github.com/paulorosantos/desphub-rpa/internal/portals/detranrs"
)

func main() {
	rpaURL := os.Getenv("RPA_URL")
	apiKey := os.Getenv("SESSAO_TOKEN_APIKEY")
	debugURL := os.Getenv("CHROME_DEBUG_URL")
	if debugURL == "" {
		debugURL = "http://localhost:9222"
	}
	if rpaURL == "" || apiKey == "" {
		fmt.Fprintln(os.Stderr, "defina RPA_URL e SESSAO_TOKEN_APIKEY")
		os.Exit(2)
	}

	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	sess, err := browser.NewRemoteSession(debugURL, browser.Options{NavTimeout: 90 * time.Second}, log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "conectar ao Chrome (%s) falhou: %v\nAbriu o Chrome com --remote-debugging-port=9222?\n", debugURL, err)
		os.Exit(1)
	}
	defer sess.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()

	log.Info("capturando token (faça o login no gov.br na janela, se pedir)...")
	auth, err := detranrs.Login(ctx, sess, log, detranrs.LoginManualTimeout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "login falhou: %v\n", err)
		os.Exit(1)
	}

	if err := empurrarToken(ctx, rpaURL, apiKey, auth); err != nil {
		fmt.Fprintf(os.Stderr, "falha ao enviar token para %s: %v\n", rpaURL, err)
		os.Exit(1)
	}
	log.Info("token enviado com sucesso ao serviço RPA", "rpaURL", rpaURL)
}

func empurrarToken(ctx context.Context, rpaURL, apiKey string, auth detranrs.Auth) error {
	body, _ := json.Marshal(map[string]string{"bearer": auth.Bearer, "userId": auth.UserID})
	url := trimBarra(rpaURL) + "/api/detran/sessao/token"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", apiKey)

	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

func trimBarra(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}
