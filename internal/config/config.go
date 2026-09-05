package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port string

	Headless      bool     
	ChromePath    string       
	NavTimeout    time.Duration 
	PageLoadWait  time.Duration

	GovBrLoginURL string // URL de login do gov.br
	DetranRSURL   string // URL base da API DETRAN/RS (vazio = DefaultBaseURL)

	LoginMode string

	ChromeDebugURL string

	LogLevel string // debug | info | warn | error

	AnonimizarPII bool
}

func Load() (Config, error) {
	cfg := Config{
		Port:          getEnv("PORT", "8080"),
		Headless:      getEnvBool("HEADLESS", true),
		ChromePath:    getEnv("CHROME_PATH", ""),
		NavTimeout:    getEnvDuration("NAV_TIMEOUT", 45*time.Second),
		PageLoadWait:  getEnvDuration("PAGE_LOAD_WAIT", 2*time.Second),
		GovBrLoginURL: getEnv("GOVBR_LOGIN_URL", "https://sso.acesso.gov.br/login"),
		DetranRSURL:   getEnv("DETRANRS_URL", ""),
		LoginMode:      getEnv("LOGIN_MODE", ""),
		ChromeDebugURL: getEnv("CHROME_DEBUG_URL", "http://localhost:9222"),
		LogLevel:      getEnv("LOG_LEVEL", "info"),
		AnonimizarPII: getEnvBool("ANONIMIZAR_PII", true),
	}

	if cfg.NavTimeout <= 0 {
		return Config{}, fmt.Errorf("config: NAV_TIMEOUT deve ser > 0")
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
