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
	apiKey := os.Getenv("SESSION_TOKEN_APIKEY")
	debugURL := os.Getenv("CHROME_DEBUG_URL")
	if debugURL == "" {
		debugURL = "http://localhost:9222"
	}
	if rpaURL == "" || apiKey == "" {
		fmt.Fprintln(os.Stderr, "set RPA_URL and SESSION_TOKEN_APIKEY")
		os.Exit(2)
	}

	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	sess, err := browser.NewRemoteSession(debugURL, browser.Options{NavTimeout: 90 * time.Second}, log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connecting to Chrome (%s) failed: %v\nDid you open Chrome with --remote-debugging-port=9222?\n", debugURL, err)
		os.Exit(1)
	}
	defer sess.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()

	log.Info("capturing token (log in to gov.br in the window if prompted)...")
	auth, err := detranrs.Login(ctx, sess, log, detranrs.LoginManualTimeout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "login failed: %v\n", err)
		os.Exit(1)
	}

	if err := pushToken(ctx, rpaURL, apiKey, auth); err != nil {
		fmt.Fprintf(os.Stderr, "failed to send token to %s: %v\n", rpaURL, err)
		os.Exit(1)
	}
	log.Info("token sent to the RPA service successfully", "rpaURL", rpaURL)
}

func pushToken(ctx context.Context, rpaURL, apiKey string, auth detranrs.Auth) error {
	body, _ := json.Marshal(map[string]string{"bearer": auth.Bearer, "userId": auth.UserID})
	url := trimSlash(rpaURL) + "/api/detran/session/token"
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

func trimSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}
