package config

import (
	"os"
)

type Config struct {
	Port      string
	JWTSecret string
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

	return &Config{
		Port:      port,
		JWTSecret: secret,
	}
}
