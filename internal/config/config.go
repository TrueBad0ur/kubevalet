package config

import (
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	Port           string `env:"PORT"           envDefault:"8080"`
	KubeconfigPath string `env:"KUBECONFIG"     envDefault:""`
	ClusterName    string `env:"CLUSTER_NAME"   envDefault:"kubernetes"`
	ClusterServer  string `env:"CLUSTER_SERVER" envDefault:""`
	// Namespace where kubevalet stores its own Secrets (private keys)
	Namespace string `env:"NAMESPACE" envDefault:"kubevalet"`
	GinMode   string `env:"GIN_MODE"  envDefault:"release"`

	// PostgreSQL
	PostgresDSN string `env:"POSTGRES_DSN" envDefault:""`

	// Auth
	JWTSecret     string        `env:"JWT_SECRET"      envDefault:"change-me-in-production"`
	TokenTTL      time.Duration `env:"TOKEN_TTL"       envDefault:"24h"`
	AdminUsername string        `env:"ADMIN_USERNAME"  envDefault:""`
	AdminPassword string        `env:"ADMIN_PASSWORD"  envDefault:""`
}

func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
