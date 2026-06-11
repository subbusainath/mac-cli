// Package credentials stores LLM API keys once per machine in
// ~/.config/mac/credentials.toml (mode 0600). Env vars always win and
// nothing key-like is ever written into a project tree.
package credentials

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Provider is a supported LLM API provider.
type Provider string

const (
	OpenAI     Provider = "openai"
	Anthropic  Provider = "anthropic"
	DeepSeek   Provider = "deepseek"
	OpenRouter Provider = "openrouter"
)

// All lists the providers the wizard asks about, in ask order.
var All = []Provider{OpenAI, Anthropic, DeepSeek, OpenRouter}

var envVars = map[Provider]string{
	OpenAI:     "OPENAI_API_KEY",
	Anthropic:  "ANTHROPIC_API_KEY",
	DeepSeek:   "DEEPSEEK_API_KEY",
	OpenRouter: "OPENROUTER_API_KEY",
}

// EnvVar returns the environment variable consulted for a provider.
func EnvVar(p Provider) string { return envVars[p] }

// Dir returns the mac config directory. MAC_CONFIG_DIR overrides the
// default ~/.config/mac (used by tests and the Python orchestrator alike).
func Dir() (string, error) {
	if d := os.Getenv("MAC_CONFIG_DIR"); d != "" {
		return d, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".config", "mac"), nil
}

func filePath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "credentials.toml"), nil
}

// Load reads stored keys. A missing file yields an empty map, not an error.
func Load() (map[Provider]string, error) {
	path, err := filePath()
	if err != nil {
		return nil, err
	}
	raw := map[string]string{}
	if _, err := toml.DecodeFile(path, &raw); err != nil {
		if os.IsNotExist(err) {
			return map[Provider]string{}, nil
		}
		return nil, fmt.Errorf("decode credentials: %w", err)
	}
	out := make(map[Provider]string, len(raw))
	for k, v := range raw {
		out[Provider(k)] = v
	}
	return out, nil
}

// Save merges keys into the credentials file, creating it with mode 0600.
func Save(keys map[Provider]string) error {
	existing, err := Load()
	if err != nil {
		return err
	}
	for p, k := range keys {
		existing[p] = k
	}

	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	path, err := filePath()
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open credentials file: %w", err)
	}
	defer f.Close()

	flat := make(map[string]string, len(existing))
	for p, k := range existing {
		flat[string(p)] = k
	}
	return toml.NewEncoder(f).Encode(flat)
}

// Lookup resolves a provider key: env var first, then the credentials file.
// source is "env" or "file". Empty values do not count.
func Lookup(p Provider) (key, source string, ok bool) {
	if v := os.Getenv(envVars[p]); v != "" {
		return v, "env", true
	}
	stored, err := Load()
	if err == nil {
		if v := stored[p]; v != "" {
			return v, "file", true
		}
	}
	return "", "", false
}
