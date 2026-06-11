package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func backendOnly() Answers {
	return Answers{Name: "demo", Backend: "fastapi",
		Planner: AgentChoice{Provider: "anthropic", Model: "claude-sonnet-4-6"},
		Coder:   AgentChoice{Provider: "local", Model: "qwen2.5-coder:14b"}}
}

func fullStack() Answers {
	return Answers{Name: "demo", Backend: "gin", Frontend: "react",
		Infra: "k8s", Cloud: "aws", IAC: "terraform",
		Planner: AgentChoice{Provider: "openai", Model: "gpt-4o"},
		Coder:   AgentChoice{Provider: "deepseek", Model: "deepseek-chat"}}
}

func read(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

func TestHarnessBackendOnlyCreatesNoDeclinedFolders(t *testing.T) {
	root := t.TempDir()
	if err := writeHarness(root, backendOnly()); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"frontend", "infra", "k8s"} {
		if _, err := os.Stat(filepath.Join(root, dir)); !os.IsNotExist(err) {
			t.Fatalf("declined folder %s was created", dir)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "backend", "CONTEXT.md")); err != nil {
		t.Fatal("backend/CONTEXT.md missing")
	}
}

func TestHarnessBackendOnlyDocsHaveNoEmptySlots(t *testing.T) {
	root := t.TempDir()
	if err := writeHarness(root, backendOnly()); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"README.md", "CONTEXT.md", "CONTEXT_MAP.md", "AGENTS.md"} {
		content := read(t, root, rel)
		if strings.Contains(content, "/ /") || strings.Contains(content, "  /") {
			t.Fatalf("%s contains empty stack slots:\n%s", rel, content)
		}
		if strings.Contains(content, "docker compose") {
			t.Fatalf("%s mentions docker compose without containers", rel)
		}
		if strings.Contains(strings.ToLower(content), "frontend/") {
			t.Fatalf("%s mentions declined frontend dir", rel)
		}
	}
}

func TestHarnessBackendOnlyQuickStartUsesStackCommand(t *testing.T) {
	root := t.TempDir()
	if err := writeHarness(root, backendOnly()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(t, root, "README.md"), "uv run uvicorn") {
		t.Fatal("README quick start should use the fastapi dev command")
	}
}

func TestHarnessFullStackMentionsEverything(t *testing.T) {
	root := t.TempDir()
	if err := writeHarness(root, fullStack()); err != nil {
		t.Fatal(err)
	}
	readme := read(t, root, "README.md")
	for _, want := range []string{"docker compose up", "k8s/", "infra/", "aws", "terraform", "gin", "react"} {
		if !strings.Contains(readme, want) {
			t.Fatalf("README missing %q", want)
		}
	}
	ctx := read(t, root, "CONTEXT.md")
	for _, want := range []string{"openai", "gpt-4o", "deepseek-chat"} {
		if !strings.Contains(ctx, want) {
			t.Fatalf("CONTEXT.md missing agent info %q", want)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "infra", "CONTEXT.md")); err != nil {
		t.Fatal("infra/CONTEXT.md missing for cloud project")
	}
}
