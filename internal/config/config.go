package config

import (
	"github.com/mohammadne/caas-operator/internal/cloudflare"
	"github.com/mohammadne/caas-operator/pkg/logger"
)

type Config struct {
	MetricsPort    int                `koanf:"metrics_port"`
	ProbePort      int                `koanf:"probe_port"`
	LeaderElection bool               `koanf:"leader_election"`
	Domain         string             `koanf:"domain"`
	LoadbalancerIP string             `koanf:"loadbalancer_ip"`
	Cloudflare     *cloudflare.Config `koanf:"cloudflare"`
	Logger         *logger.Config     `koanf:"logger"`
}
