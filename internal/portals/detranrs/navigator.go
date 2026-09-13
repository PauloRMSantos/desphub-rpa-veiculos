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
	VehicleQueryURL = "https://pcsdetran.rs.gov.br/consulta-veiculo?contabiliza=true"
	apiHostMatch    = "procergs.com.br"

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
		raw := headerValue(e.Request.Headers, "authorization")
		token := strings.TrimSpace(strings.TrimPrefix(raw, "Bearer "))
		if !looksLikeJWT(token) {
			log.Debug("procergs call without a valid token yet", "hasAuthHeader", raw != "")
			return
		}
		mu.Lock()
		captured.Bearer = token
		if uid := headerValue(e.Request.Headers, "x-user-id"); uid != "" {
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
			if e := chromedp.Navigate(VehicleQueryURL).Do(c); e != nil {
				log.Warn("initial navigation returned an error (likely /login redirect); continuing", "error", e.Error())
			}
			log.Info("browser opened at the DETRAN-RS login page")
			log.Info("=======================================================")
			log.Info(">> ACTION REQUIRED: log in to gov.br in the Chrome window that opened.")
			log.Info(">> The RPA takes over automatically once you finish the login.")
			log.Info("=======================================================", "waitingUpTo", timeout.String())
			return nil
		}),
		chromedp.ActionFunc(func(c context.Context) error {
			select {
			case <-done:
				return nil
			case <-time.After(timeout):
				return fmt.Errorf("timed out waiting for token capture (%s)", timeout)
			case <-c.Done():
				return c.Err()
			}
		}),
	)
	if err != nil {
		return Auth{}, fmt.Errorf("detranrs: manual login: %w", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if captured.UserID == "" {
		log.Warn("token captured, but X-User-Id did not appear — confirm in the real flow")
	}
	log.Info("login detected; session credentials captured successfully")
	return captured, nil
}

func looksLikeJWT(token string) bool {
	if len(token) < 20 {
		return false
	}
	return strings.Count(token, ".") >= 2 && strings.HasPrefix(token, "ey")
}

func headerValue(h network.Headers, key string) string {
	key = strings.ToLower(key)
	for k, v := range h {
		if strings.ToLower(k) == key {
			if s, ok := v.(string); ok {
				return s
			}
			return fmt.Sprintf("%v", v)
		}
	}
	return ""
}
