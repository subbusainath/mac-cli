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
	Infra    string `toml:"infra,omitempty"`
	Cloud    string `toml:"cloud,omitempty"`
	IAC      string `toml:"iac,omitempty"`
}

type AgentConfig struct {
	Provider string `toml:"provider"`
	Model    string `toml:"model"`
	APIBase  string `toml:"api_base,omitempty"`
}

// Default builds the project config. Zero-valued planner/coder fall back
// to local Ollama so a key-less setup still works.
func Default(name, backend, frontend, infra, cloud, iac string,
	planner, coder AgentConfig) *Config {
	return &Config{
		Project: ProjectConfig{
			Name:     name,
			Backend:  backend,
			Frontend: frontend,
			Infra:    infra,
			Cloud:    cloud,
			IAC:      iac,
		},
		Agents: map[string]AgentConfig{
			"planner": orLocal(planner, "qwen2.5-coder:14b"),
			"coder":   orLocal(coder, "qwen2.5-coder:14b"),
		},
	}
}

func orLocal(a AgentConfig, defaultModel string) AgentConfig {
	if a.Provider == "" {
		return AgentConfig{Provider: "local", Model: defaultModel,
			APIBase: "http://localhost:11434"}
	}
	if a.Provider == "local" && a.APIBase == "" {
		a.APIBase = "http://localhost:11434"
	}
	return a
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
