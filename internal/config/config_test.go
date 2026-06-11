package config

import (
	"testing"
)

func TestWriteReadRoundTripWithAgentsAndInfra(t *testing.T) {
	dir := t.TempDir()
	cfg := Default("proj", "fastapi", "react", "k8s", "aws", "terraform",
		AgentConfig{Provider: "anthropic", Model: "claude-sonnet-4-6"},
		AgentConfig{Provider: "deepseek", Model: "deepseek-chat"})
	if err := Write(dir, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Project.Infra != "k8s" {
		t.Fatalf("infra = %q", got.Project.Infra)
	}
	if got.Agents["planner"].Provider != "anthropic" {
		t.Fatalf("planner = %+v", got.Agents["planner"])
	}
	if got.Agents["coder"].Provider != "deepseek" || got.Agents["coder"].Model != "deepseek-chat" {
		t.Fatalf("coder = %+v", got.Agents["coder"])
	}
}

func TestDefaultFallsBackToLocalAgents(t *testing.T) {
	cfg := Default("p", "", "", "", "", "", AgentConfig{}, AgentConfig{})
	if cfg.Agents["planner"].Provider != "local" || cfg.Agents["coder"].Provider != "local" {
		t.Fatalf("agents = %+v", cfg.Agents)
	}
	if cfg.Agents["coder"].APIBase == "" {
		t.Fatal("local coder needs api_base")
	}
}
