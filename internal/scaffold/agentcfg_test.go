package scaffold

import "testing"

func TestAgentCfgLocalDefaults(t *testing.T) {
	got := agentCfg(AgentChoice{})
	if got.Provider != "local" || got.Model != "qwen2.5-coder:14b" || got.APIBase != "http://localhost:11434" {
		t.Fatalf("got %+v", got)
	}
}

func TestAgentCfgCloudProviderPassthrough(t *testing.T) {
	got := agentCfg(AgentChoice{Provider: "openai", Model: "gpt-4o"})
	if got.Provider != "openai" || got.Model != "gpt-4o" || got.APIBase != "" {
		t.Fatalf("got %+v", got)
	}
}
