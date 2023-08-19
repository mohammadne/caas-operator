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
		Domain:         "mohammadne.me",
		LoadbalancerIP: "86.104.38.209",
		Cloudflare: &cloudflare.Config{
			CloudflareURL: "https://api.cloudflare.com/client/v4",
			Token:         "-H6dnfFIJTTu3IS97vofwuZ5TlncLN6S2mUu9PEn",
			ZoneID:        "68cae808906f3e60c657c6c2e53db98f",
		},
		Logger: &logger.Config{
			Development: true,
			Level:       "info",
			Encoding:    "console",
		},
	}
}
