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
	switch cfg.LoginMode {
	case "token":
		portal := detranrs.NewTokenPortal(cfg.DetranRSURL)
		svc = consulta.NewService(portal, log, cfg.AnonimizarPII)
		if cfg.SessaoTokenAPIKey == "" {
			log.Warn("LOGIN_MODE=token sem SESSAO_TOKEN_APIKEY: endpoint de token ficará SEM proteção")
		}
		log.Info("portal DETRAN-RS em modo TOKEN (injeção externa via POST /api/detran/sessao/token)")
	case "manual":
		portal := detranrs.NewSessionPortal(cfg.ChromeDebugURL, browser.Options{
			NavTimeout:   cfg.NavTimeout,
			PageLoadWait: cfg.PageLoadWait,
		}, cfg.DetranRSURL, log)
		svc = consulta.NewService(portal, log, cfg.AnonimizarPII)
		log.Info("portal DETRAN-RS em modo LOGIN MANUAL (reconecta ao Chrome por login)",
			"chromeDebugURL", cfg.ChromeDebugURL)
	default:
		log.Warn("LOGIN_MODE vazio: defina 'token' (VPS) ou 'manual' (local); consultas responderão 501")
	}

	mux := http.NewServeMux()
	api.NewServer(log, svc, cfg.SessaoTokenAPIKey).Routes(mux)

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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("shutdown falhou", "erro", err.Error())
	}
	log.Info("serviço encerrado")
}
