package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/paulorosantos/desphub-rpa/internal/api"
	"github.com/paulorosantos/desphub-rpa/internal/browser"
	"github.com/paulorosantos/desphub-rpa/internal/config"
	"github.com/paulorosantos/desphub-rpa/internal/consulta"
	"github.com/paulorosantos/desphub-rpa/internal/logger"
	"github.com/paulorosantos/desphub-rpa/internal/portals/detranrs"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		os.Stderr.WriteString("erro ao carregar config: " + err.Error() + "\n")
		os.Exit(1)
	}

	log := logger.New(cfg.LogLevel)
	log.Info("iniciando desphub-rpa",
		"port", cfg.Port,
		"headless", cfg.Headless,
		"navTimeout", cfg.NavTimeout.String(),
		"anonimizarPII", cfg.AnonimizarPII,
	)

	var svc *consulta.Service
	var sess *browser.Session
	if cfg.LoginMode == "manual" {
		// Conecta-se ao Chrome do operador (aberto com --remote-debugging-port).
		// Navegador real evita a detecção de bot que invalida o captcha do gov.br.
		s, err := browser.NewRemoteSession(cfg.ChromeDebugURL, browser.Options{
			NavTimeout:   cfg.NavTimeout,
			PageLoadWait: cfg.PageLoadWait,
		}, log)
		if err != nil {
			log.Error("não foi possível conectar ao Chrome de depuração; /api/v1/consultas responderá 501",
				"chromeDebugURL", cfg.ChromeDebugURL, "erro", err.Error())
		} else {
			sess = s
			portal := detranrs.NewSessionPortal(sess, cfg.DetranRSURL, log)
			svc = consulta.NewService(portal, log, cfg.AnonimizarPII)
			log.Info("portal DETRAN-RS em modo LOGIN MANUAL (attach ao Chrome do operador)",
				"chromeDebugURL", cfg.ChromeDebugURL)
		}
	} else {
		log.Warn("LOGIN_MODE não é 'manual': /api/v1/consultas responderá 501")
	}

	mux := http.NewServeMux()
	api.NewServer(log, svc).Routes(mux)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Info("servidor HTTP no ar", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("servidor HTTP falhou", "erro", err.Error())
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Info("encerrando serviço...")
	if sess != nil {
		sess.Close()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("shutdown falhou", "erro", err.Error())
	}
	log.Info("serviço encerrado")
}
