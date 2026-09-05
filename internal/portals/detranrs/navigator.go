package detranrs

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"

	"github.com/paulorosantos/desphub-rpa/internal/browser"
	"github.com/paulorosantos/desphub-rpa/internal/logger"
)

const (
	ConsultaURL  = "https://pcsdetran.rs.gov.br/consulta-veiculo?contabiliza=true"
	apiHostMatch = "procergs.com.br"

	LoginManualTimeout = 5 * time.Minute
	LoginSilentTimeout = 25 * time.Second
)

func Login(ctx context.Context, sess *browser.Session, log *slog.Logger, timeout time.Duration) (Auth, error) {
	log = logger.FromContext(ctx, log)
	if timeout <= 0 {
		timeout = LoginManualTimeout
	}

	var (
		mu       sync.Mutex
		captured Auth
		done     = make(chan struct{})
		once     sync.Once
	)

	chromedp.ListenTarget(sess.Ctx(), func(ev interface{}) {
		e, ok := ev.(*network.EventRequestWillBeSent)
		if !ok || !strings.Contains(e.Request.URL, apiHostMatch) {
			return
		}
		raw := headerValor(e.Request.Headers, "authorization")
		token := strings.TrimSpace(strings.TrimPrefix(raw, "Bearer "))
		if !pareceJWT(token) {
			log.Debug("chamada à procergs sem token válido ainda", "temAuthHeader", raw != "")
			return
		}
		mu.Lock()
		captured.Bearer = token
		if uid := headerValor(e.Request.Headers, "x-user-id"); uid != "" {
			captured.UserID = uid
		}
		mu.Unlock()
		once.Do(func() { close(done) })
	})

	runCtx, cancel := context.WithTimeout(sess.Ctx(), timeout+30*time.Second)
	defer cancel()
	go func() {
		select {
		case <-ctx.Done():
			cancel()
		case <-done:
		}
	}()

	err := chromedp.Run(runCtx,
		network.Enable(),
		chromedp.ActionFunc(func(c context.Context) error {
			if e := chromedp.Navigate(ConsultaURL).Do(c); e != nil {
				log.Warn("navegação inicial retornou erro (provável redirect ao /login); seguindo", "erro", e.Error())
			}
			log.Info("navegador aberto na página de login do DETRAN-RS")
			log.Info("=======================================================")
			log.Info(">> AÇÃO NECESSÁRIA: faça o LOGIN no gov.br na janela do Chrome que abriu.")
			log.Info(">> O RPA assume automaticamente assim que você concluir o login.")
			log.Info("=======================================================",
				"aguardandoAte", timeout.String())
			return nil
		}),
		chromedp.ActionFunc(func(c context.Context) error {
			select {
			case <-done:
				return nil
			case <-time.After(timeout):
				return fmt.Errorf("tempo esgotado aguardando captura do token (%s)", timeout)
			case <-c.Done():
				return c.Err()
			}
		}),
	)
	if err != nil {
		return Auth{}, fmt.Errorf("detranrs: login manual: %w", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if captured.UserID == "" {
		log.Warn("token capturado, mas X-User-Id não apareceu — confirmar no fluxo real")
	}
	log.Info("login detectado; credenciais de sessão capturadas com sucesso")
	return captured, nil
}

func pareceJWT(token string) bool {
	if len(token) < 20 {
		return false
	}
	return strings.Count(token, ".") >= 2 && strings.HasPrefix(token, "ey")
}

func headerValor(h network.Headers, chave string) string {
	chave = strings.ToLower(chave)
	for k, v := range h {
		if strings.ToLower(k) == chave {
			if s, ok := v.(string); ok {
				return s
			}
			return fmt.Sprintf("%v", v)
		}
	}
	return ""
}
