package detranrs

import (
	"context"
	"encoding/base64"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/paulorosantos/desphub-rpa/internal/model"
)

func TestTokenPortalWithoutTokenNeedsReconnect(t *testing.T) {
	p := NewTokenPortal("")
	_, err := p.Query(context.Background(), "ABC1D23", "123")
	if !errors.Is(err, model.ErrReconnectRequired) {
		t.Fatalf("without token expected ErrReconnectRequired, got %v", err)
	}
}

func TestTokenPortalSetTokenStatus(t *testing.T) {
	exp := time.Now().Add(30 * time.Minute).Unix()
	jwt := fakeJWT(exp)

	p := NewTokenPortal("")
	p.SetToken(jwt, "user-123")

	authed, expiresAt := p.Status()
	if !authed {
		t.Fatal("expected authenticated after SetToken")
	}
	if expiresAt.Unix() != exp {
		t.Errorf("expiresAt = %d; want %d", expiresAt.Unix(), exp)
	}
}

func TestJwtExpiryInvalid(t *testing.T) {
	if got := jwtExpiry("no-dots"); !got.IsZero() {
		t.Errorf("invalid token should be zero, got %v", got)
	}
}

// fakeJWT builds a synthetic JWT with the exp claim (signature irrelevant).
func fakeJWT(exp int64) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"exp":` + strconv.FormatInt(exp, 10) + `}`))
	return header + "." + payload + ".sig"
}
