package config

import (
	"github.com/mohammadne/caas-operator/internal/cloudflare"
	"github.com/mohammadne/caas-operator/pkg/logger"
)

func defaultConfig() *Config {
	return &Config{
		MetricsPort:    8080,
		ProbePort:      8081,
		LeaderElection: true,
		Cloudflare: &cloudflare.Config{
			CloudflareURL: "https://api.cloudflare.com/client/v4",
		},
		Logger: &logger.Config{
			Development: true,
			Level:       "info",
			Encoding:    "console",
		},
	}
}
