package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	BindHost string
	Port     string

	Headless     bool
	ChromePath   string
	NavTimeout   time.Duration
	PageLoadWait time.Duration

	GovBrLoginURL string 
	DetranRSURL   string 

	LoginMode string

	ChromeDebugURL string

	SessionTokenAPIKey string

	LogLevel string // debug | info | warn | error

	AnonymizePII bool
}

func Load() (Config, error) {
	cfg := Config{
		BindHost:           getEnv("BIND_HOST", "127.0.0.1"),
		Port:               getEnv("PORT", "8080"),
		Headless:           getEnvBool("HEADLESS", true),
		ChromePath:         getEnv("CHROME_PATH", ""),
		NavTimeout:         getEnvDuration("NAV_TIMEOUT", 45*time.Second),
		PageLoadWait:       getEnvDuration("PAGE_LOAD_WAIT", 2*time.Second),
		GovBrLoginURL:      getEnv("GOVBR_LOGIN_URL", "https://sso.acesso.gov.br/login"),
		DetranRSURL:        getEnv("DETRANRS_URL", ""),
		LoginMode:          getEnv("LOGIN_MODE", ""),
		ChromeDebugURL:     getEnv("CHROME_DEBUG_URL", "http://localhost:9222"),
		SessionTokenAPIKey: getEnv("SESSION_TOKEN_APIKEY", ""),
		LogLevel:           getEnv("LOG_LEVEL", "info"),
		AnonymizePII:       getEnvBool("ANONYMIZE_PII", true),
	}

	if cfg.NavTimeout <= 0 {
		return Config{}, fmt.Errorf("config: NAV_TIMEOUT must be > 0")
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
