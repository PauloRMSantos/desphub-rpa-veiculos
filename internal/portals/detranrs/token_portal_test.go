package detranrs

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/paulorosantos/desphub-rpa/internal/model"
)

func TestTokenPortalSemTokenPedeReconexao(t *testing.T) {
	p := NewTokenPortal("")
	_, err := p.Consultar(context.Background(), "ABC1D23", "123")
	if !errors.Is(err, model.ErrReconexaoNecessaria) {
		t.Fatalf("sem token esperava ErrReconexaoNecessaria, veio %v", err)
	}
}

func TestTokenPortalSetTokenStatus(t *testing.T) {
	exp := time.Now().Add(30 * time.Minute).Unix()
	jwt := fakeJWT(exp)

	p := NewTokenPortal("")
	p.SetToken(jwt, "user-123")

	authed, expiraEm := p.Status()
	if !authed {
		t.Fatal("esperava autenticado após SetToken")
	}
	if expiraEm.Unix() != exp {
		t.Errorf("expiraEm = %d; quero %d", expiraEm.Unix(), exp)
	}
}

func TestJwtExpiracaoInvalido(t *testing.T) {
	if got := jwtExpiracao("sem-pontos"); !got.IsZero() {
		t.Errorf("token inválido deveria dar zero, veio %v", got)
	}
}

// fakeJWT monta um JWT sintético com o claim exp (assinatura irrelevante).
func fakeJWT(exp int64) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(
		`{"exp":` + itoa64(exp) + `}`))
	return header + "." + payload + ".sig"
}

func itoa64(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}
