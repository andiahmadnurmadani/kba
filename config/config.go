package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server  ServerConfig   `yaml:"server"`
	Backup  BackupConfig   `yaml:"backup"`
	Storage StorageConfig  `yaml:"storage"`
}

type ServerConfig struct {
	Name string `yaml:"name"`
}

type BackupConfig struct {
	MySQL     string `yaml:"mysql"`
	Postgres  string `yaml:"postgres"`
	MongoDB   string `yaml:"mongodb"`
	Nginx     string `yaml:"nginx"`
	PM2       string `yaml:"pm2"`
	Docker    string `yaml:"docker"`
	SSL       string `yaml:"ssl"`
	Git       string `yaml:"git"`
	Cron      string `yaml:"cron"`
}

type StorageConfig struct {
	Type        string `yaml:"type"`
	Destination string `yaml:"destination"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Defaults
	if cfg.Storage.Type == "" {
		cfg.Storage.Type = "local"
	}
	if cfg.Storage.Destination == "" {
		cfg.Storage.Destination = "/var/backups/kroombox"
	}
	if cfg.Server.Name == "" {
		hostname, _ := os.Hostname()
		cfg.Server.Name = hostname
	}

	return &cfg, nil
}

func DefaultConfig() *Config {
	hostname, _ := os.Hostname()
	return &Config{
		Server: ServerConfig{Name: hostname},
		Backup: BackupConfig{
			MySQL:    "auto",
			Postgres: "auto",
			MongoDB:  "auto",
			Nginx:    "auto",
			PM2:      "auto",
			Docker:   "auto",
			SSL:      "auto",
			Git:      "auto",
			Cron:     "auto",
		},
		Storage: StorageConfig{
			Type:        "local",
			Destination: "/var/backups/kroombox",
		},
	}
}

