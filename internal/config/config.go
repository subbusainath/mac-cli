package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const (
	dirName  = ".mac"
	fileName = "config.toml"
)

type Config struct {
	Project ProjectConfig          `toml:"project"`
	Agents  map[string]AgentConfig `toml:"agents"`
}

type ProjectConfig struct {
	Name     string `toml:"name"`
	Backend  string `toml:"backend"`
	Frontend string `toml:"frontend"`
	Cloud    string `toml:"cloud"`
	IAC      string `toml:"iac"`
}

type AgentConfig struct {
	Provider string `toml:"provider"`
	Model    string `toml:"model"`
	APIBase  string `toml:"api_base,omitempty"`
}

func Default(name, backend, frontend, cloud, iac string) *Config {
	return &Config{
		Project: ProjectConfig{
			Name:     name,
			Backend:  backend,
			Frontend: frontend,
			Cloud:    cloud,
			IAC:      iac,
		},
		Agents: map[string]AgentConfig{
			"planner": {
				Provider: "anthropic",
				Model:    "claude-sonnet-4-6",
			},
			"coder": {
				Provider: "local",
				Model:    "qwen2.5-coder:14b",
				APIBase:  "http://localhost:11434",
			},
		},
	}
}

func Write(projectRoot string, cfg *Config) error {
	dir := filepath.Join(projectRoot, dirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create .mac dir: %w", err)
	}
	f, err := os.Create(filepath.Join(dir, fileName))
	if err != nil {
		return fmt.Errorf("create config file: %w", err)
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(cfg)
}

func Read(projectRoot string) (*Config, error) {
	path := filepath.Join(projectRoot, dirName, fileName)
	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	return &cfg, nil
}
