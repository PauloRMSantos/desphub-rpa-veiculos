package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/paulorosantos/desphub-rpa/internal/api"
	"github.com/paulorosantos/desphub-rpa/internal/browser"
	"github.com/paulorosantos/desphub-rpa/internal/config"
	"github.com/paulorosantos/desphub-rpa/internal/logger"
	"github.com/paulorosantos/desphub-rpa/internal/portals/detranrs"
	"github.com/paulorosantos/desphub-rpa/internal/portals/detransc"
	"github.com/paulorosantos/desphub-rpa/internal/query"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		os.Stderr.WriteString("error loading config: " + err.Error() + "\n")
		os.Exit(1)
	}

	log := logger.New(cfg.LogLevel)
	log.Info("starting desphub-rpa",
		"port", cfg.Port,
		"loginMode", cfg.LoginMode,
		"navTimeout", cfg.NavTimeout.String(),
		"anonymizePII", cfg.AnonymizePII,
	)

	var portal query.Portal
	switch cfg.LoginMode {
	case "token":
		portal = detranrs.NewTokenPortal(cfg.DetranRSURL)
		if cfg.SessionTokenAPIKey == "" {
			log.Warn("LOGIN_MODE=token without SESSION_TOKEN_APIKEY: the token endpoint will be UNPROTECTED")
		}
		log.Info("DETRAN-RS portal in TOKEN mode (external injection via POST /api/detran/session/token)")
	case "manual":
		portal = detranrs.NewSessionPortal(cfg.ChromeDebugURL, browser.Options{
			NavTimeout:   cfg.NavTimeout,
			PageLoadWait: cfg.PageLoadWait,
		}, cfg.DetranRSURL, log)
		log.Info("DETRAN-RS portal in MANUAL LOGIN mode (reconnects to Chrome per login)",
			"chromeDebugURL", cfg.ChromeDebugURL)
	default:
		log.Warn("LOGIN_MODE empty: DETRAN-RS disabled (state=RS returns 501); DETRAN-SC ingest still available")
	}

	svc := query.NewService(portal, log, cfg.AnonymizePII)
	svc.RegisterParser("SC", detransc.ParseVehicle)

	mux := http.NewServeMux()
	api.NewServer(log, svc, cfg.SessionTokenAPIKey).Routes(mux)

	srv := &http.Server{
		Addr:              net.JoinHostPort(cfg.BindHost, cfg.Port),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Info("HTTP server up", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("HTTP server failed", "error", err.Error())
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Info("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("shutdown failed", "error", err.Error())
	}
	log.Info("service stopped")
}
