package config

import (
	"os"
	"strings"
)

type Config struct {
	Port            string
	JWTSecret       string
	AllowedOrigins  []string
	AuthServiceAddr string
}

func Load() *Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = ":8080"
	} else if port[0] != ':' {
		port = ":" + port
	}

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "supersecret"
	}

	originsStr := os.Getenv("ALLOWED_ORIGINS")
	var allowedOrigins []string
	if originsStr != "" {
		allowedOrigins = strings.Split(originsStr, ",")
		for i, origin := range allowedOrigins {
			allowedOrigins[i] = strings.TrimSpace(origin)
		}
	} else {
		allowedOrigins = []string{"*"}
	}

	authServiceAddr := os.Getenv("AUTH_SERVICE_ADDR")
	if authServiceAddr == "" {
		authServiceAddr = "auth-service:50051"
	}

	return &Config{
		Port:            port,
		JWTSecret:       secret,
		AllowedOrigins:  allowedOrigins,
		AuthServiceAddr: authServiceAddr,
	}
}

