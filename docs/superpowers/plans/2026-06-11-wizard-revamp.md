# Wizard Revamp & Code TUI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Optional infra/cloud wizard gates, LLM API-key collection with global credentials, conditional scaffold/docs (no unwanted folders), JSON event protocol between Go CLI and Python orchestrator, Bubble Tea TUI for `mac code`, animated MAC banner.

**Architecture:** Go side: new `internal/credentials` package, choice-aware scaffold generators keyed off an extended `scaffold.Answers`, reworked wizard steps, new `coderun` and `banner` TUI models. Python side: provider/key resolution (`credentials.py`), expanded `llm.py` provider map, `ui.py` JSON-lines protocol driven from `__main__.py` using LangGraph `stream_mode=["updates","messages"]`.

**Tech Stack:** Go (cobra, bubbletea, lipgloss, BurntSushi/toml — all existing; new: `github.com/aymanbagabas/go-udiff`, `github.com/alecthomas/chroma/v2`), Python 3.12 (stdlib `tomllib`, existing langgraph/langchain).

**Spec:** `docs/superpowers/specs/2026-06-11-wizard-revamp-design.md`

**Conventions:** Module path `github.com/subbusainath/mac-cli`. Go tests: `go test ./...` from repo root. Python tests: `cd orchestrator && uv run pytest -v`. Every task ends in a commit. The repo registered project config `.mac/` at repo root stays untracked.

---

### Task 1: Go credentials package

**Files:**
- Create: `internal/credentials/credentials.go`
- Test: `internal/credentials/credentials_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package credentials

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	t.Setenv("MAC_CONFIG_DIR", t.TempDir())
	if err := Save(map[Provider]string{OpenAI: "sk-test-123"}); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got[OpenAI] != "sk-test-123" {
		t.Fatalf("got %q", got[OpenAI])
	}
}

func TestSaveMergesExistingKeys(t *testing.T) {
	t.Setenv("MAC_CONFIG_DIR", t.TempDir())
	if err := Save(map[Provider]string{OpenAI: "sk-a"}); err != nil {
		t.Fatal(err)
	}
	if err := Save(map[Provider]string{DeepSeek: "sk-b"}); err != nil {
		t.Fatal(err)
	}
	got, _ := Load()
	if got[OpenAI] != "sk-a" || got[DeepSeek] != "sk-b" {
		t.Fatalf("merge lost keys: %v", got)
	}
}

func TestSaveFileMode0600(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MAC_CONFIG_DIR", dir)
	if err := Save(map[Provider]string{Anthropic: "sk-c"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, "credentials.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestLoadMissingFileIsEmpty(t *testing.T) {
	t.Setenv("MAC_CONFIG_DIR", t.TempDir())
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("want empty, got %v", got)
	}
}

func TestLookupEnvWinsOverFile(t *testing.T) {
	t.Setenv("MAC_CONFIG_DIR", t.TempDir())
	if err := Save(map[Provider]string{OpenAI: "from-file"}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENAI_API_KEY", "from-env")
	key, source, ok := Lookup(OpenAI)
	if !ok || key != "from-env" || source != "env" {
		t.Fatalf("got %q %q %v", key, source, ok)
	}
}

func TestLookupFallsBackToFile(t *testing.T) {
	t.Setenv("MAC_CONFIG_DIR", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "") // empty env must not count
	if err := Save(map[Provider]string{OpenAI: "from-file"}); err != nil {
		t.Fatal(err)
	}
	key, source, ok := Lookup(OpenAI)
	if !ok || key != "from-file" || source != "file" {
		t.Fatalf("got %q %q %v", key, source, ok)
	}
}

func TestLookupMissing(t *testing.T) {
	t.Setenv("MAC_CONFIG_DIR", t.TempDir())
	t.Setenv("DEEPSEEK_API_KEY", "")
	if _, _, ok := Lookup(DeepSeek); ok {
		t.Fatal("want not found")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/credentials/ -v`
Expected: FAIL — package does not exist / undefined symbols.

- [ ] **Step 3: Implement `credentials.go`**

```go
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/credentials/ -v`
Expected: all 7 PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/credentials/
git commit -m "feat(credentials): global API key store with env precedence"
```

---

### Task 2: Answers extension + config agents plumbing

**Files:**
- Modify: `internal/scaffold/scaffold.go` (Answers struct, `New`, new helper `agentCfg`)
- Modify: `internal/config/config.go` (`ProjectConfig.Infra`, `Default` signature)
- Test: `internal/config/config_test.go`, `internal/scaffold/agentcfg_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/config/config_test.go`:

```go
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
```

`internal/scaffold/agentcfg_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ ./internal/scaffold/ -run 'TestWriteRead|TestDefault|TestAgentCfg' -v`
Expected: FAIL — wrong `Default` arity, undefined `AgentChoice`, `agentCfg`.

- [ ] **Step 3: Implement**

In `internal/config/config.go`, add `Infra` to `ProjectConfig`:

```go
type ProjectConfig struct {
	Name     string `toml:"name"`
	Backend  string `toml:"backend"`
	Frontend string `toml:"frontend"`
	Infra    string `toml:"infra,omitempty"`
	Cloud    string `toml:"cloud,omitempty"`
	IAC      string `toml:"iac,omitempty"`
}
```

Replace `Default` with:

```go
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
```

In `internal/scaffold/scaffold.go`, replace the `Answers` struct and add `agentCfg`:

```go
// AgentChoice is the wizard's provider+model pick for one agent role.
type AgentChoice struct {
	Provider string
	Model    string
}

// Answers holds the choices collected by the wizard. Empty string means
// the user declined that option.
type Answers struct {
	Name     string
	Path     string
	Backend  string
	Frontend string
	Infra    string // "" | "local" | "containers" | "k8s"
	Cloud    string // "" when declined or Infra declined
	IAC      string // "" when no cloud
	Planner  AgentChoice
	Coder    AgentChoice
	Keys     map[string]string // provider -> key pasted in wizard; saved globally, never in the project
}

func agentCfg(c AgentChoice) config.AgentConfig {
	if c.Provider == "" || c.Provider == "local" {
		model := c.Model
		if model == "" {
			model = "qwen2.5-coder:14b"
		}
		return config.AgentConfig{Provider: "local", Model: model,
			APIBase: "http://localhost:11434"}
	}
	return config.AgentConfig{Provider: c.Provider, Model: c.Model}
}
```

In `scaffold.New`, change the config line to:

```go
	cfg := config.Default(a.Name, a.Backend, a.Frontend, a.Infra, a.Cloud, a.IAC,
		agentCfg(a.Planner), agentCfg(a.Coder))
```

The wizard does not compile yet against new fields — that is fine; it sets none of the new fields until Task 6. Verify `go build ./...` still passes (it will: new fields are additive, `Default` call updated).

- [ ] **Step 4: Run tests + build**

Run: `go build ./... && go test ./internal/config/ ./internal/scaffold/ -v`
Expected: build OK; new tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/ internal/scaffold/
git commit -m "feat(scaffold): infra/agent answers and config agents plumbing"
```

---

### Task 3: Conditional harness docs — no unwanted folders

**Files:**
- Modify: `internal/scaffold/harness.go` (all template builders become choice-aware)
- Test: `internal/scaffold/harness_test.go`

- [ ] **Step 1: Write the failing tests**

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/scaffold/ -run TestHarness -v`
Expected: FAIL — current templates hardcode all sections and always write all three CONTEXT.md files.

- [ ] **Step 3: Rewrite `harness.go` template builders**

Replace `agentsMD`, `contextMapMD`, `contextMD`, `readmeMD`, `subContextFiles` with choice-aware versions. Keep `writeHarness`, run-command maps as-is.

```go
// stackParts lists only the options the user actually chose.
func stackParts(a Answers) []string {
	var parts []string
	if a.Backend != "" {
		parts = append(parts, "backend: "+a.Backend)
	}
	if a.Frontend != "" {
		parts = append(parts, "frontend: "+a.Frontend)
	}
	if a.Infra != "" {
		parts = append(parts, "infra: "+a.Infra)
	}
	if a.Cloud != "" {
		parts = append(parts, fmt.Sprintf("cloud: %s (%s)", a.Cloud, a.IAC))
	}
	parts = append(parts,
		fmt.Sprintf("planner: %s/%s", a.Planner.Provider, a.Planner.Model),
		fmt.Sprintf("coder: %s/%s", a.Coder.Provider, a.Coder.Model))
	return parts
}

// quickStart returns the single command that brings the project up.
func quickStart(a Answers) string {
	if a.Infra == "containers" || a.Infra == "k8s" {
		return "docker compose up"
	}
	if a.Backend != "" {
		return "cd backend && " + backendRunCmds[a.Backend]
	}
	if a.Frontend != "" {
		return "cd frontend && " + frontendRunCmds[a.Frontend]
	}
	return "# no runnable components scaffolded"
}
```

`agentsMD` — replace the `Stack:` line construction:

```go
func agentsMD(a Answers) string {
	return fmt.Sprintf(`# AGENTS.md — Global Guardrails

Stack: %s  |  Generated: %s
`+agentsBodyMD,
		strings.Join(stackParts(a), " · "),
		time.Now().Format("2006-01-02"),
	)
}
```

where `agentsBodyMD` is a `const` holding the existing Golden Principles / TDD Rules / Casing / Prologue sections verbatim (move them out of the old Sprintf).

`contextMapMD` — build rows conditionally:

```go
func contextMapMD(a Answers) string {
	type row struct{ dir, purpose, ctx string }
	rows := []row{}
	if a.Backend != "" {
		rows = append(rows, row{"backend/", a.Backend + " service — Hexagonal Architecture", "backend/CONTEXT.md"})
	}
	if a.Frontend != "" {
		rows = append(rows, row{"frontend/", a.Frontend + " UI layer", "frontend/CONTEXT.md"})
	}
	if a.Cloud != "" {
		rows = append(rows, row{"infra/", fmt.Sprintf("%s / %s infrastructure", a.Cloud, a.IAC), "infra/CONTEXT.md"})
	}
	if a.Infra == "k8s" {
		rows = append(rows, row{"k8s/", "Kubernetes manifests", "—"})
	}
	rows = append(rows,
		row{"docs/", "Architecture diagrams, ADRs", "—"},
		row{"scripts/", "Dev tooling, CI helpers", "—"},
		row{".mac/", "mac CLI config (config.toml)", "—"})

	var b strings.Builder
	b.WriteString("# CONTEXT_MAP.md — Navigation Index\n\n")
	b.WriteString("| Directory | Purpose | Context file |\n|-----------|---------|--------------|\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "| %s | %s | %s |\n", r.dir, r.purpose, r.ctx)
	}
	return b.String()
}
```

`contextMD`:

```go
func contextMD(a Answers) string {
	var b strings.Builder
	b.WriteString("# CONTEXT.md — Root\n\n## Truth Tier: Authoritative\n\n## Stack\n\n")
	for _, p := range stackParts(a) {
		b.WriteString("- " + p + "\n")
	}
	b.WriteString("\n## Run Commands\n\n```bash\n" + quickStart(a) + "\n")
	if a.Backend != "" {
		b.WriteString("# Backend only  →  see backend/CONTEXT.md\n")
	}
	if a.Frontend != "" {
		b.WriteString("# Frontend only →  see frontend/CONTEXT.md\n")
	}
	b.WriteString("```\n\n## Active Plans\n\n- [ ] Initial scaffold complete\n")
	return b.String()
}
```

`readmeMD` — conditional mermaid + tree:

```go
func readmeMD(a Answers) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n> %s\n\n## Quick Start\n\n```bash\n%s\n```\n",
		a.Name, strings.Join(stackParts(a), " · "), quickStart(a))

	b.WriteString("\n## Architecture\n\n```mermaid\ngraph TB\n")
	if a.Frontend != "" {
		fmt.Fprintf(&b, "    subgraph FE [\"%s Frontend\"]\n        UI[UI Layer]\n    end\n", a.Frontend)
	}
	if a.Backend != "" {
		fmt.Fprintf(&b, "    subgraph BE [\"%s Backend — Hexagonal\"]\n", a.Backend)
		b.WriteString("        API[Adapters / API]\n        APP[Application / Use Cases]\n")
		b.WriteString("        DOM[Domain / Entities]\n        PER[Adapters / Persistence]\n    end\n")
	}
	if a.Frontend != "" && a.Backend != "" {
		b.WriteString("    UI --> API\n")
	}
	if a.Backend != "" {
		b.WriteString("    API --> APP\n    APP --> DOM\n    APP --> PER\n")
	}
	b.WriteString("```\n\n## Directory Layout\n\n```\n.\n")
	if a.Backend != "" {
		fmt.Fprintf(&b, "├── backend/   # %s (Hexagonal Architecture)\n", a.Backend)
	}
	if a.Frontend != "" {
		fmt.Fprintf(&b, "├── frontend/  # %s\n", a.Frontend)
	}
	if a.Cloud != "" {
		fmt.Fprintf(&b, "├── infra/     # %s / %s\n", a.Cloud, a.IAC)
	}
	if a.Infra == "k8s" {
		b.WriteString("├── k8s/       # Kubernetes manifests\n")
	}
	b.WriteString("├── docs/\n└── .mac/      # mac CLI config\n```\n")
	return b.String()
}
```

`subContextFiles` — conditional:

```go
func subContextFiles(a Answers) map[string]string {
	out := map[string]string{}
	if a.Backend != "" {
		out["backend"] = fmt.Sprintf(`# CONTEXT.md — backend

## Architecture: Hexagonal

Layers (innermost → outermost): Domain → Application → Adapters

## Run

`+"```"+`bash
cd backend && %s
`+"```"+`

## Test

`+"```"+`bash
cd backend && %s
`+"```"+`
`, backendRunCmds[a.Backend], backendTestCmds[a.Backend])
	}
	if a.Frontend != "" {
		out["frontend"] = fmt.Sprintf(`# CONTEXT.md — frontend

## Framework: %s

## Run

`+"```"+`bash
cd frontend && %s
`+"```"+`
`, a.Frontend, frontendRunCmds[a.Frontend])
	}
	if a.Cloud != "" {
		out["infra"] = `# CONTEXT.md — infra

## Contains IaC configuration only — no application logic.

## Apply

` + "```" + `bash
# See the subdirectory for your chosen tool (terraform / cdk / sam / pulumi / bicep)
` + "```" + `
`
	}
	return out
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/scaffold/ -run TestHarness -v`
Expected: all 4 PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/scaffold/harness.go internal/scaffold/harness_test.go
git commit -m "feat(scaffold): choice-aware docs, no folders for declined options"
```

---

### Task 4: Kubernetes manifest generator

**Files:**
- Create: `internal/scaffold/k8s.go`
- Test: `internal/scaffold/k8s_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteK8sBackendAndFrontend(t *testing.T) {
	root := t.TempDir()
	a := Answers{Name: "demo", Backend: "gin", Frontend: "react", Infra: "k8s"}
	if err := writeK8s(root, a); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{
		"k8s/backend-deployment.yaml", "k8s/backend-service.yaml",
		"k8s/frontend-deployment.yaml", "k8s/frontend-service.yaml",
		"k8s/kustomization.yaml",
	} {
		if _, err := os.Stat(filepath.Join(root, f)); err != nil {
			t.Fatalf("missing %s", f)
		}
	}
	kust, _ := os.ReadFile(filepath.Join(root, "k8s", "kustomization.yaml"))
	if !strings.Contains(string(kust), "backend-deployment.yaml") ||
		!strings.Contains(string(kust), "frontend-service.yaml") {
		t.Fatalf("kustomization incomplete:\n%s", kust)
	}
}

func TestWriteK8sBackendOnly(t *testing.T) {
	root := t.TempDir()
	a := Answers{Name: "demo", Backend: "fastapi", Infra: "k8s"}
	if err := writeK8s(root, a); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "k8s", "frontend-deployment.yaml")); !os.IsNotExist(err) {
		t.Fatal("frontend manifests must not exist")
	}
	dep, _ := os.ReadFile(filepath.Join(root, "k8s", "backend-deployment.yaml"))
	if !strings.Contains(string(dep), "demo-backend") {
		t.Fatalf("deployment missing app name:\n%s", dep)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/scaffold/ -run TestWriteK8s -v`
Expected: FAIL — `writeK8s` undefined.

- [ ] **Step 3: Implement `k8s.go`**

```go
package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Backend containers listen on 8000; frontends on 3000. Matches the ports
// already used by writeDockerfiles.
func writeK8s(root string, a Answers) error {
	dir := filepath.Join(root, "k8s")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir k8s: %w", err)
	}

	var resources []string
	write := func(name, content string) error {
		resources = append(resources, name)
		return os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
	}

	if a.Backend != "" {
		if err := write("backend-deployment.yaml", k8sDeployment(a.Name, "backend", 8000)); err != nil {
			return err
		}
		if err := write("backend-service.yaml", k8sService(a.Name, "backend", 8000)); err != nil {
			return err
		}
	}
	if a.Frontend != "" {
		if err := write("frontend-deployment.yaml", k8sDeployment(a.Name, "frontend", 3000)); err != nil {
			return err
		}
		if err := write("frontend-service.yaml", k8sService(a.Name, "frontend", 3000)); err != nil {
			return err
		}
	}

	kust := "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources:\n"
	for _, r := range resources {
		kust += "  - " + r + "\n"
	}
	return os.WriteFile(filepath.Join(dir, "kustomization.yaml"), []byte(kust), 0o644)
}

func k8sDeployment(project, component string, port int) string {
	app := fmt.Sprintf("%s-%s", strings.ToLower(project), component)
	return fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: %[1]s
  labels:
    app: %[1]s
spec:
  replicas: 1
  selector:
    matchLabels:
      app: %[1]s
  template:
    metadata:
      labels:
        app: %[1]s
    spec:
      containers:
        - name: %[2]s
          image: %[1]s:latest
          imagePullPolicy: IfNotPresent
          ports:
            - containerPort: %[3]d
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 500m
              memory: 512Mi
`, app, component, port)
}

func k8sService(project, component string, port int) string {
	app := fmt.Sprintf("%s-%s", strings.ToLower(project), component)
	return fmt.Sprintf(`apiVersion: v1
kind: Service
metadata:
  name: %[1]s
spec:
  selector:
    app: %[1]s
  ports:
    - port: 80
      targetPort: %[2]d
`, app, port)
}
```

(If `writeDockerfiles` exposes different ports per stack, reuse those constants instead — check `internal/scaffold/docker.go` and align.)

- [ ] **Step 4: Run tests**

Run: `go test ./internal/scaffold/ -run TestWriteK8s -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/scaffold/k8s.go internal/scaffold/k8s_test.go
git commit -m "feat(scaffold): kubernetes manifest generator for k8s infra tier"
```

---

### Task 5: Gate docker/k8s/cloud generation in `scaffold.New`

**Files:**
- Modify: `internal/scaffold/scaffold.go:26-59` (`New`)

- [ ] **Step 1: Update `New`**

Replace the unconditional `writeDockerfiles` / `writeCloudIaC` calls:

```go
	if a.Infra == "containers" || a.Infra == "k8s" {
		if err := writeDockerfiles(root, a.Backend, a.Frontend); err != nil {
			return err
		}
	}
	if a.Infra == "k8s" {
		if err := writeK8s(root, a); err != nil {
			return err
		}
	}
	if a.Cloud != "" {
		if err := writeCloudIaC(root, a.Cloud, a.IAC); err != nil {
			return err
		}
	}
```

Also check `writeCloudIaC` (`internal/scaffold/cloud.go`) and `writeDockerfiles` (`internal/scaffold/docker.go`) for any internal unconditional folder creation (e.g. `infra/` mkdir before the switch) — they must create folders only along the path actually taken. If `writeDockerfiles` writes `docker-compose.yml` referencing declined components, gate those blocks on `backend != ""` / `frontend != ""` (it already takes both as params — verify and fix if needed).

- [ ] **Step 2: Build + full test run**

Run: `go build ./... && go test ./...`
Expected: PASS (harness/k8s/config/credentials tests all green).

- [ ] **Step 3: Commit**

```bash
git add internal/scaffold/
git commit -m "feat(scaffold): generate docker/k8s/cloud only when chosen"
```

---

### Task 6: Wizard — infra/cloud gates, API keys, agent picks

**Files:**
- Modify: `internal/tui/wizard.go`
- Test: `internal/tui/wizard_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/subbusainath/mac-cli/internal/credentials"
)

func cleanEnv(t *testing.T) {
	t.Helper()
	t.Setenv("MAC_CONFIG_DIR", t.TempDir())
	for _, p := range credentials.All {
		t.Setenv(credentials.EnvVar(p), "")
	}
}

func pick(t *testing.T, l *list.Model, label string) {
	t.Helper()
	for i, it := range l.Items() {
		if it.(choiceItem).label == label {
			l.Select(i)
			return
		}
	}
	t.Fatalf("label %q not in list", label)
}

// drive answers name+path and the backend/frontend gates with "no".
func driveToInfra(t *testing.T) wizardModel {
	t.Helper()
	m := newWizardModel("/tmp")
	m.nameInput.SetValue("demo")
	mm, _ := m.advance() // name
	m = mm.(wizardModel)
	mm, _ = m.advance() // path (default cwd)
	m = mm.(wizardModel)
	pick(t, &m.wantBackend, "no")
	mm, _ = m.advance()
	m = mm.(wizardModel)
	pick(t, &m.wantFrontend, "no")
	mm, _ = m.advance()
	return mm.(wizardModel)
}

func TestInfraNoSkipsCloudAndGoesToKeys(t *testing.T) {
	cleanEnv(t)
	m := driveToInfra(t)
	if m.step != stepWantInfra {
		t.Fatalf("step = %v, want stepWantInfra", m.step)
	}
	pick(t, &m.wantInfra, "no")
	mm, _ := m.advance()
	m = mm.(wizardModel)
	if m.step != stepKeyGate {
		t.Fatalf("step = %v, want stepKeyGate (skip infra target, cloud, iac)", m.step)
	}
	if m.Answers.Infra != "" || m.Answers.Cloud != "" || m.Answers.IAC != "" {
		t.Fatalf("declined infra must clear: %+v", m.Answers)
	}
}

func TestCloudNoSkipsIaC(t *testing.T) {
	cleanEnv(t)
	m := driveToInfra(t)
	pick(t, &m.wantInfra, "yes")
	mm, _ := m.advance()
	m = mm.(wizardModel)
	if m.step != stepInfraTarget {
		t.Fatalf("step = %v", m.step)
	}
	pick(t, &m.infraTarget, "containers")
	mm, _ = m.advance()
	m = mm.(wizardModel)
	if m.step != stepWantCloud {
		t.Fatalf("step = %v", m.step)
	}
	pick(t, &m.wantCloud, "no")
	mm, _ = m.advance()
	m = mm.(wizardModel)
	if m.step != stepKeyGate {
		t.Fatalf("step = %v, want stepKeyGate", m.step)
	}
	if m.Answers.Infra != "containers" || m.Answers.Cloud != "" {
		t.Fatalf("answers: %+v", m.Answers)
	}
}

func TestAllKeysDeclinedLeavesOnlyLocal(t *testing.T) {
	cleanEnv(t)
	m := driveToInfra(t)
	pick(t, &m.wantInfra, "no")
	mm, _ := m.advance()
	m = mm.(wizardModel)
	// Decline all four providers.
	for i := 0; i < 4; i++ {
		if m.step != stepKeyGate {
			t.Fatalf("iteration %d: step = %v", i, m.step)
		}
		pick(t, &m.keyGate, "no")
		mm, _ = m.advance()
		m = mm.(wizardModel)
	}
	if m.step != stepPlanner {
		t.Fatalf("step = %v, want stepPlanner", m.step)
	}
	items := m.plannerPick.Items()
	if len(items) != 1 || items[0].(choiceItem).label != "local" {
		t.Fatalf("planner choices = %v, want only local", items)
	}
}

func TestDetectedKeySkipsGate(t *testing.T) {
	cleanEnv(t)
	t.Setenv("ANTHROPIC_API_KEY", "sk-present")
	m := driveToInfra(t)
	pick(t, &m.wantInfra, "no")
	mm, _ := m.advance()
	m = mm.(wizardModel)
	// First gate must be openai (anthropic auto-detected, openai asked first anyway);
	// decline remaining gates and confirm anthropic appears in planner list.
	for m.step == stepKeyGate {
		pick(t, &m.keyGate, "no")
		mm, _ = m.advance()
		m = mm.(wizardModel)
	}
	if m.step != stepPlanner {
		t.Fatalf("step = %v", m.step)
	}
	var labels []string
	for _, it := range m.plannerPick.Items() {
		labels = append(labels, it.(choiceItem).label)
	}
	if labels[0] != "anthropic" {
		t.Fatalf("planner default should be anthropic (strongest with key), got %v", labels)
	}
}

func TestFullKeyFlowThroughConfirm(t *testing.T) {
	cleanEnv(t)
	m := driveToInfra(t)
	pick(t, &m.wantInfra, "no")
	mm, _ := m.advance()
	m = mm.(wizardModel)
	// Say yes to openai, paste key, decline rest.
	pick(t, &m.keyGate, "yes")
	mm, _ = m.advance()
	m = mm.(wizardModel)
	if m.step != stepKeyInput {
		t.Fatalf("step = %v", m.step)
	}
	m.keyInput.SetValue("sk-pasted")
	mm, _ = m.advance()
	m = mm.(wizardModel)
	for m.step == stepKeyGate {
		pick(t, &m.keyGate, "no")
		mm, _ = m.advance()
		m = mm.(wizardModel)
	}
	if m.Answers.Keys["openai"] != "sk-pasted" {
		t.Fatalf("keys = %v", m.Answers.Keys)
	}
	// planner: openai picked by default
	mm, _ = m.advance() // accept planner provider
	m = mm.(wizardModel)
	if m.step != stepPlannerModel {
		t.Fatalf("step = %v", m.step)
	}
	if m.plannerModel.Value() != "gpt-4o" {
		t.Fatalf("planner model default = %q", m.plannerModel.Value())
	}
	mm, _ = m.advance() // accept model
	m = mm.(wizardModel)
	mm, _ = m.advance() // accept coder provider (local default)
	m = mm.(wizardModel)
	mm, _ = m.advance() // accept coder model
	m = mm.(wizardModel)
	if m.step != stepConfirm {
		t.Fatalf("step = %v, want stepConfirm", m.step)
	}
	if m.Answers.Planner.Provider != "openai" || m.Answers.Planner.Model != "gpt-4o" {
		t.Fatalf("planner = %+v", m.Answers.Planner)
	}
	if m.Answers.Coder.Provider != "local" {
		t.Fatalf("coder = %+v", m.Answers.Coder)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/ -run 'TestInfra|TestCloud|TestAllKeys|TestDetected|TestFullKey' -v`
Expected: FAIL — undefined steps/fields.

- [ ] **Step 3: Implement wizard changes**

3a. Replace the step enum and metadata (drop `maxStepCount`):

```go
const (
	stepName wizardStep = iota
	stepPath
	stepWantBackend
	stepBackend
	stepWantFrontend
	stepFrontend
	stepWantInfra
	stepInfraTarget
	stepWantCloud
	stepCloud
	stepIaC
	stepKeyGate
	stepKeyInput
	stepPlanner
	stepPlannerModel
	stepCoder
	stepCoderModel
	stepConfirm
)
```

Update `stepNames` / `stepLabels` / `progressOrder` accordingly (progress shows: Name, Path, Backend?, Frontend?, Infra?, Cloud?, LLM Keys, Agents, Confirm — group key/agent steps under two slots: use `progressOrder` of `stepName, stepPath, stepWantBackend, stepWantFrontend, stepWantInfra, stepWantCloud, stepKeyGate, stepPlanner`, and map detail steps to their gate's slot in `renderProgress` via a `progressSlot(step) wizardStep` helper: `stepBackend→stepWantBackend`, `stepFrontend→stepWantFrontend`, `stepInfraTarget→stepWantInfra`, `stepCloud, stepIaC→stepWantCloud`, `stepKeyInput→stepKeyGate`, `stepPlannerModel, stepCoder, stepCoderModel→stepPlanner`).

3b. New choice lists and defaults:

```go
var infraTargetChoices = []list.Item{
	choiceItem{"local", "Run on this machine only — no container files"},
	choiceItem{"containers", "Dockerfiles + docker-compose"},
	choiceItem{"k8s", "Containers + Kubernetes manifests"},
}

var defaultModels = map[string]string{
	"openai":     "gpt-4o",
	"anthropic":  "claude-sonnet-4-6",
	"deepseek":   "deepseek-chat",
	"openrouter": "openrouter/auto",
	"local":      "qwen2.5-coder:14b",
}

var providerTitles = map[credentials.Provider]string{
	credentials.OpenAI:     "OpenAI",
	credentials.Anthropic:  "Claude (Anthropic)",
	credentials.DeepSeek:   "DeepSeek",
	credentials.OpenRouter: "OpenRouter",
}

// plannerOrder: strongest first; coderOrder: cheapest first.
var plannerOrder = []string{"anthropic", "openai", "openrouter", "deepseek", "local"}
var coderOrder = []string{"local", "deepseek", "openrouter", "openai", "anthropic"}
```

3c. Model fields added to `wizardModel`:

```go
	wantInfra    list.Model
	infraTarget  list.Model
	wantCloud    list.Model
	keyGate      list.Model
	keyInput     textinput.Model
	keyIdx       int
	keyStatus    map[credentials.Provider]string // "env" | "file" | "wizard"
	plannerPick  list.Model
	plannerModel textinput.Model
	coderPick    list.Model
	coderModel   textinput.Model
```

Init in `newWizardModel`:

```go
	key := newStyledInput()
	key.EchoMode = textinput.EchoPassword
	key.CharLimit = 256

	model := wizardModel{
		// ... existing fields ...
		wantInfra:    newChoiceList(yesNoItems, "Include infrastructure?"),
		infraTarget:  newChoiceList(infraTargetChoices, "Deployment target"),
		wantCloud:    newChoiceList(yesNoItems, "Deploy to cloud?"),
		keyInput:     key,
		keyStatus:    map[credentials.Provider]string{},
		plannerModel: newStyledInput(),
		coderModel:   newStyledInput(),
	}
	model.Answers.Keys = map[string]string{}
	return model
```

3d. Key-gate iteration helper (pointer receiver — mutate then assign in `advance`):

```go
// enterKeyPhase advances keyIdx past providers whose key is already known
// and returns the next step: another gate, or the planner pick.
func (m *wizardModel) enterKeyPhase() wizardStep {
	for m.keyIdx < len(credentials.All) {
		p := credentials.All[m.keyIdx]
		if _, src, ok := credentials.Lookup(p); ok {
			m.keyStatus[p] = src
			m.keyIdx++
			continue
		}
		title := fmt.Sprintf("Have a %s API key?", providerTitles[p])
		m.keyGate = newChoiceList(yesNoItems, title)
		return stepKeyGate
	}
	m.buildAgentLists()
	return stepPlanner
}

// availableProviders returns providers with keys, ordered by pref, plus local.
func (m *wizardModel) availableProviders(order []string) []list.Item {
	var items []list.Item
	for _, prov := range order {
		if prov == "local" {
			items = append(items, choiceItem{"local",
				"Ollama at localhost:11434 — " + defaultModels["local"]})
			continue
		}
		if m.keyStatus[credentials.Provider(prov)] != "" {
			items = append(items, choiceItem{prov, "default: " + defaultModels[prov]})
		}
	}
	return items
}

func (m *wizardModel) buildAgentLists() {
	m.plannerPick = newChoiceList(m.availableProviders(plannerOrder), "Planner model provider")
	m.coderPick = newChoiceList(m.availableProviders(coderOrder), "Coder model provider")
}
```

3e. `advance()` — replace the `stepCloud`/`stepIaC` cases and add new ones:

```go
	case stepWantInfra:
		item, ok := m.wantInfra.SelectedItem().(choiceItem)
		if !ok {
			return m, nil
		}
		if item.label == "yes" {
			m.step = stepInfraTarget
		} else {
			m.Answers.Infra, m.Answers.Cloud, m.Answers.IAC = "", "", ""
			m.step = m.enterKeyPhase()
		}

	case stepInfraTarget:
		item, ok := m.infraTarget.SelectedItem().(choiceItem)
		if !ok {
			return m, nil
		}
		m.Answers.Infra = item.label
		m.step = stepWantCloud

	case stepWantCloud:
		item, ok := m.wantCloud.SelectedItem().(choiceItem)
		if !ok {
			return m, nil
		}
		if item.label == "yes" {
			m.step = stepCloud
		} else {
			m.Answers.Cloud, m.Answers.IAC = "", ""
			m.step = m.enterKeyPhase()
		}

	case stepCloud:
		item, ok := m.cloud.SelectedItem().(choiceItem)
		if !ok {
			return m, nil
		}
		m.Answers.Cloud = item.label
		m.iac = newChoiceList(iacByCloud[item.label], "Select IaC tool")
		m.step = stepIaC

	case stepIaC:
		item, ok := m.iac.SelectedItem().(choiceItem)
		if !ok {
			return m, nil
		}
		m.Answers.IAC = item.label
		m.step = m.enterKeyPhase()

	case stepKeyGate:
		item, ok := m.keyGate.SelectedItem().(choiceItem)
		if !ok {
			return m, nil
		}
		p := credentials.All[m.keyIdx]
		if item.label == "yes" {
			m.keyInput.SetValue("")
			m.keyInput.Placeholder = credentials.EnvVar(p) + " value"
			m.keyInput.Focus()
			m.step = stepKeyInput
		} else {
			m.keyIdx++
			m.step = m.enterKeyPhase()
		}

	case stepKeyInput:
		p := credentials.All[m.keyIdx]
		if val := strings.TrimSpace(m.keyInput.Value()); val != "" {
			m.Answers.Keys[string(p)] = val
			m.keyStatus[p] = "wizard"
		}
		m.keyInput.Blur()
		m.keyIdx++
		m.step = m.enterKeyPhase()

	case stepPlanner:
		item, ok := m.plannerPick.SelectedItem().(choiceItem)
		if !ok {
			return m, nil
		}
		m.Answers.Planner.Provider = item.label
		m.plannerModel.SetValue(defaultModels[item.label])
		m.plannerModel.Focus()
		m.step = stepPlannerModel

	case stepPlannerModel:
		m.Answers.Planner.Model = strings.TrimSpace(m.plannerModel.Value())
		if m.Answers.Planner.Model == "" {
			m.Answers.Planner.Model = defaultModels[m.Answers.Planner.Provider]
		}
		m.plannerModel.Blur()
		m.step = stepCoder

	case stepCoder:
		item, ok := m.coderPick.SelectedItem().(choiceItem)
		if !ok {
			return m, nil
		}
		m.Answers.Coder.Provider = item.label
		m.coderModel.SetValue(defaultModels[item.label])
		m.coderModel.Focus()
		m.step = stepCoderModel

	case stepCoderModel:
		m.Answers.Coder.Model = strings.TrimSpace(m.coderModel.Value())
		if m.Answers.Coder.Model == "" {
			m.Answers.Coder.Model = defaultModels[m.Answers.Coder.Provider]
		}
		m.coderModel.Blur()
		m.step = stepConfirm
```

Note `advance()` has a value receiver; `enterKeyPhase` mutates — call as `m.step = (&m).enterKeyPhase()` works since `m` is an addressable local; plain `m.enterKeyPhase()` auto-takes the address. Keep that.

Also update the `stepWantFrontend`/`stepFrontend` cases to go to `stepWantInfra` instead of `stepCloud`.

3f. `Update()` — route messages to new components (mirror existing pattern):

```go
	case stepWantInfra:
		m.wantInfra, cmd = m.wantInfra.Update(msg)
	case stepInfraTarget:
		m.infraTarget, cmd = m.infraTarget.Update(msg)
	case stepWantCloud:
		m.wantCloud, cmd = m.wantCloud.Update(msg)
	case stepKeyGate:
		m.keyGate, cmd = m.keyGate.Update(msg)
	case stepKeyInput:
		m.keyInput, cmd = m.keyInput.Update(msg)
	case stepPlanner:
		m.plannerPick, cmd = m.plannerPick.Update(msg)
	case stepPlannerModel:
		m.plannerModel, cmd = m.plannerModel.Update(msg)
	case stepCoder:
		m.coderPick, cmd = m.coderPick.Update(msg)
	case stepCoderModel:
		m.coderModel, cmd = m.coderModel.Update(msg)
```

In `WindowSizeMsg` handling add `SetWidth` for the new lists (guard `keyGate`/`plannerPick`/`coderPick` behind step checks like `iac`).

3g. `View()` — new cases:

```go
	case stepWantInfra:
		b.WriteString(renderYesNoStep("Include infrastructure (containers / K8s / cloud)?", m.wantInfra))
	case stepInfraTarget:
		b.WriteString(renderListStep(m.infraTarget, "Deployment Target"))
	case stepWantCloud:
		b.WriteString(renderYesNoStep("Deploy to a cloud provider?", m.wantCloud))
	case stepKeyGate:
		p := credentials.All[m.keyIdx]
		b.WriteString(renderKeyStatus(m.keyStatus))
		b.WriteString(renderYesNoStep(fmt.Sprintf("Have a %s API key?", providerTitles[p]), m.keyGate))
	case stepKeyInput:
		p := credentials.All[m.keyIdx]
		b.WriteString(renderTextStep(fmt.Sprintf("Paste your %s key (input hidden)", providerTitles[p]), m.keyInput.View()))
	case stepPlanner:
		b.WriteString(renderListStep(m.plannerPick, "Planner Provider"))
	case stepPlannerModel:
		b.WriteString(renderTextStep("Planner model (edit or press enter)", m.plannerModel.View()))
	case stepCoder:
		b.WriteString(renderListStep(m.coderPick, "Coder Provider"))
	case stepCoderModel:
		b.WriteString(renderTextStep("Coder model (edit or press enter)", m.coderModel.View()))
```

```go
// renderKeyStatus shows ✓ badges for providers already resolved.
func renderKeyStatus(status map[credentials.Provider]string) string {
	var parts []string
	for _, p := range credentials.All {
		if src := status[p]; src != "" {
			parts = append(parts, CheckStyle("✓ ")+string(p)+DimStyle(" ("+src+")"))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return lipgloss.NewStyle().PaddingLeft(2).Render(strings.Join(parts, "   ")) + "\n\n"
}
```

3h. `back()` — extend the gate mapping:

```go
	switch m.step {
	case stepBackend:
		m.step = stepWantBackend
	case stepFrontend:
		m.step = stepWantFrontend
	case stepInfraTarget:
		m.step = stepWantInfra
	case stepCloud:
		m.step = stepWantCloud
	case stepKeyInput:
		m.step = stepKeyGate
	case stepKeyGate, stepPlanner:
		m.step = stepWantInfra // key phase entry points vary; infra gate is the stable anchor
	case stepPlannerModel:
		m.step = stepPlanner
	case stepCoder:
		m.step = stepPlannerModel
	case stepCoderModel:
		m.step = stepCoder
	default:
		m.step--
	}
```

(Backing out of the key phase resets nothing destructive; re-entry re-runs detection. Reset `m.keyIdx = 0` and clear `m.keyStatus` entries whose value is not `"wizard"` when leaving via this path.)

3i. `renderConfirm` — extend rows:

```go
	rows := []struct{ k, v string }{
		{"Project", a.Name},
		{"Path", a.Path},
		{"Backend", iface(a.Backend)},
		{"Frontend", iface(a.Frontend)},
		{"Infra", iface(a.Infra)},
		{"Cloud", iface(a.Cloud)},
		{"IaC", iface(a.IAC)},
		{"Planner", a.Planner.Provider + " / " + a.Planner.Model},
		{"Coder", a.Coder.Provider + " / " + a.Coder.Model},
	}
```

- [ ] **Step 4: Run tests**

Run: `go build ./... && go test ./internal/tui/ -v`
Expected: all wizard tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/
git commit -m "feat(wizard): infra/cloud gates, API key collection, agent provider picks"
```

---

### Task 7: Launcher saves credentials + conditional summary

**Files:**
- Modify: `internal/tui/launcher.go:71-103`

- [ ] **Step 1: Implement**

Before `scaffold.New(...)` insert:

```go
	if len(wiz.Answers.Keys) > 0 {
		keys := make(map[credentials.Provider]string, len(wiz.Answers.Keys))
		for p, k := range wiz.Answers.Keys {
			keys[credentials.Provider(p)] = k
		}
		if err := credentials.Save(keys); err != nil {
			return fmt.Errorf("save credentials: %w", err)
		}
	}
```

Replace the hardcoded `Stack:` printf with conditional output:

```go
	var stack []string
	for _, part := range []string{wiz.Answers.Backend, wiz.Answers.Frontend,
		wiz.Answers.Infra, wiz.Answers.Cloud, wiz.Answers.IAC} {
		if part != "" {
			stack = append(stack, part)
		}
	}
	fmt.Printf("  %s  %s\n", DimStyle("Stack:"), strings.Join(stack, " / "))
	fmt.Printf("  %s  planner %s/%s · coder %s/%s\n",
		DimStyle("Agents:"),
		wiz.Answers.Planner.Provider, wiz.Answers.Planner.Model,
		wiz.Answers.Coder.Provider, wiz.Answers.Coder.Model)
```

Add `"github.com/subbusainath/mac-cli/internal/credentials"` to imports.

- [ ] **Step 2: Build + test**

Run: `go build ./... && go test ./...`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/tui/launcher.go
git commit -m "feat(launcher): persist wizard keys globally, conditional summary"
```

---

### Task 8: Python credentials + provider map

**Files:**
- Create: `orchestrator/src/mac_orchestrator/credentials.py`
- Modify: `orchestrator/src/mac_orchestrator/llm.py`
- Test: `orchestrator/tests/test_credentials.py`, extend `orchestrator/tests/test_llm.py`

- [ ] **Step 1: Write the failing tests**

`orchestrator/tests/test_credentials.py`:

```python
import pytest

from mac_orchestrator.credentials import lookup_key, require_key


def test_env_wins(monkeypatch, tmp_path):
    monkeypatch.setenv("MAC_CONFIG_DIR", str(tmp_path))
    (tmp_path / "credentials.toml").write_text('openai = "from-file"\n')
    monkeypatch.setenv("OPENAI_API_KEY", "from-env")
    assert lookup_key("openai") == "from-env"


def test_file_fallback(monkeypatch, tmp_path):
    monkeypatch.setenv("MAC_CONFIG_DIR", str(tmp_path))
    monkeypatch.delenv("OPENAI_API_KEY", raising=False)
    (tmp_path / "credentials.toml").write_text('openai = "from-file"\n')
    assert lookup_key("openai") == "from-file"


def test_missing_returns_none(monkeypatch, tmp_path):
    monkeypatch.setenv("MAC_CONFIG_DIR", str(tmp_path))
    monkeypatch.delenv("DEEPSEEK_API_KEY", raising=False)
    assert lookup_key("deepseek") is None


def test_empty_env_does_not_count(monkeypatch, tmp_path):
    monkeypatch.setenv("MAC_CONFIG_DIR", str(tmp_path))
    monkeypatch.setenv("OPENROUTER_API_KEY", "")
    assert lookup_key("openrouter") is None


def test_require_key_raises_with_guidance(monkeypatch, tmp_path):
    monkeypatch.setenv("MAC_CONFIG_DIR", str(tmp_path))
    monkeypatch.delenv("ANTHROPIC_API_KEY", raising=False)
    with pytest.raises(RuntimeError, match="ANTHROPIC_API_KEY"):
        require_key("anthropic")
```

Extend `orchestrator/tests/test_llm.py` (keep existing tests; match their style for asserting model construction — they already verify `ChatOpenAI` base_url for local):

```python
def test_openai_provider(monkeypatch):
    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")
    model = build_chat_model(AgentLLM(provider="openai", model="gpt-4o"))
    assert model.model_name == "gpt-4o"


def test_deepseek_provider(monkeypatch):
    monkeypatch.setenv("DEEPSEEK_API_KEY", "sk-test")
    model = build_chat_model(AgentLLM(provider="deepseek", model="deepseek-chat"))
    assert "api.deepseek.com" in str(model.openai_api_base)


def test_openrouter_provider(monkeypatch):
    monkeypatch.setenv("OPENROUTER_API_KEY", "sk-test")
    model = build_chat_model(AgentLLM(provider="openrouter", model="openrouter/auto"))
    assert "openrouter.ai" in str(model.openai_api_base)


def test_missing_key_raises(monkeypatch, tmp_path):
    monkeypatch.setenv("MAC_CONFIG_DIR", str(tmp_path))
    monkeypatch.delenv("DEEPSEEK_API_KEY", raising=False)
    with pytest.raises(RuntimeError, match="DEEPSEEK_API_KEY"):
        build_chat_model(AgentLLM(provider="deepseek", model="deepseek-chat"))
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd orchestrator && uv run pytest tests/test_credentials.py tests/test_llm.py -v`
Expected: FAIL — `credentials` module missing; unknown providers raise ValueError.

- [ ] **Step 3: Implement**

`credentials.py`:

```python
"""Resolve provider API keys: env var first, then ~/.config/mac/credentials.toml.

Mirrors the Go internal/credentials package, including the MAC_CONFIG_DIR
test override.
"""
import os
import tomllib
from pathlib import Path

ENV_VARS = {
    "openai": "OPENAI_API_KEY",
    "anthropic": "ANTHROPIC_API_KEY",
    "deepseek": "DEEPSEEK_API_KEY",
    "openrouter": "OPENROUTER_API_KEY",
}


def _credentials_path() -> Path:
    base = os.environ.get("MAC_CONFIG_DIR")
    root = Path(base) if base else Path.home() / ".config" / "mac"
    return root / "credentials.toml"


def lookup_key(provider: str) -> str | None:
    env = ENV_VARS.get(provider)
    if env:
        val = os.environ.get(env, "").strip()
        if val:
            return val
    path = _credentials_path()
    if path.exists():
        data = tomllib.loads(path.read_text())
        val = str(data.get(provider, "")).strip()
        if val:
            return val
    return None


def require_key(provider: str) -> str:
    key = lookup_key(provider)
    if key is None:
        env = ENV_VARS.get(provider, "<unknown>")
        raise RuntimeError(
            f"no API key for provider '{provider}'. Set {env} or store the key "
            f"in {_credentials_path()} (re-running the 'mac' wizard can do this)."
        )
    return key
```

`llm.py` becomes:

```python
"""Map .mac/config.toml agent entries to chat models.

'anthropic'  -> Claude via resolved key
'openai'     -> OpenAI via resolved key
'deepseek'   -> DeepSeek's OpenAI-compatible API
'openrouter' -> OpenRouter's OpenAI-compatible API
'local'      -> any OpenAI-compatible server (Ollama) at api_base
"""
from langchain_anthropic import ChatAnthropic
from langchain_openai import ChatOpenAI

from mac_orchestrator.config import AgentLLM
from mac_orchestrator.credentials import require_key

_DEFAULT_OLLAMA = "http://localhost:11434"

_OPENAI_COMPAT_BASES = {
    "deepseek": "https://api.deepseek.com/v1",
    "openrouter": "https://openrouter.ai/api/v1",
}


def build_chat_model(cfg: AgentLLM):
    if cfg.provider == "anthropic":
        return ChatAnthropic(model=cfg.model, max_tokens=8192,
                             api_key=require_key("anthropic"))
    if cfg.provider == "openai":
        return ChatOpenAI(model=cfg.model, api_key=require_key("openai"))
    if cfg.provider in _OPENAI_COMPAT_BASES:
        return ChatOpenAI(model=cfg.model,
                          base_url=_OPENAI_COMPAT_BASES[cfg.provider],
                          api_key=require_key(cfg.provider))
    if cfg.provider == "local":
        base = (cfg.api_base or _DEFAULT_OLLAMA).rstrip("/")
        return ChatOpenAI(model=cfg.model, base_url=base + "/v1", api_key="ollama")
    raise ValueError(f"unknown provider: {cfg.provider}")
```

Check existing `tests/test_llm.py` for an anthropic test that relied on `ANTHROPIC_API_KEY` implicitly — set it via monkeypatch now that the key is resolved eagerly.

- [ ] **Step 4: Run tests**

Run: `cd orchestrator && uv run pytest -v`
Expected: full suite PASS.

- [ ] **Step 5: Commit**

```bash
git add orchestrator/src/mac_orchestrator/credentials.py orchestrator/src/mac_orchestrator/llm.py orchestrator/tests/
git commit -m "feat(orchestrator): provider key resolution and openai/deepseek/openrouter support"
```

---

### Task 9: Python JSON UI protocol

**Files:**
- Create: `orchestrator/src/mac_orchestrator/ui.py`
- Modify: `orchestrator/src/mac_orchestrator/apply.py` (add `changes_with_old`)
- Modify: `orchestrator/src/mac_orchestrator/__main__.py` (`--ui` flag, event loop)
- Test: `orchestrator/tests/test_ui.py`, extend `orchestrator/tests/test_apply.py`

- [ ] **Step 1: Write the failing tests**

`orchestrator/tests/test_ui.py`:

```python
import io
import json

from mac_orchestrator.ui import JsonUI, PlainUI


def test_json_emit_one_line(capsys):
    ui = JsonUI()
    ui.emit(event="phase", phase="RED", iterations=1)
    out = capsys.readouterr().out
    lines = out.strip().splitlines()
    assert len(lines) == 1
    assert json.loads(lines[0]) == {"event": "phase", "phase": "RED", "iterations": 1}


def test_json_read_command(monkeypatch):
    monkeypatch.setattr("sys.stdin", io.StringIO('{"cmd": "feedback", "text": "use a dict"}\n'))
    assert JsonUI().read_command() == {"cmd": "feedback", "text": "use a dict"}


def test_json_read_eof_quits(monkeypatch):
    monkeypatch.setattr("sys.stdin", io.StringIO(""))
    assert JsonUI().read_command() == {"cmd": "quit"}


def test_json_read_garbage_quits(monkeypatch):
    monkeypatch.setattr("sys.stdin", io.StringIO("not json\n"))
    assert JsonUI().read_command() == {"cmd": "quit"}


def test_plain_approve(monkeypatch, capsys):
    monkeypatch.setattr("builtins.input", lambda _: "a")
    ui = PlainUI()
    ui.emit(event="await_approval", changes=[{"path": "x.py", "old": "", "new": "pass\n"}])
    assert ui.read_command() == {"cmd": "approve"}


def test_plain_feedback(monkeypatch):
    answers = iter(["f", "rename it"])
    monkeypatch.setattr("builtins.input", lambda _: next(answers))
    assert PlainUI().read_command() == {"cmd": "feedback", "text": "rename it"}
```

Add to `orchestrator/tests/test_apply.py`:

```python
from mac_orchestrator.apply import changes_with_old


def test_changes_with_old_reads_existing(tmp_path):
    (tmp_path / "a.py").write_text("old\n")
    out = changes_with_old(str(tmp_path), [
        {"path": "a.py", "content": "new\n"},
        {"path": "b.py", "content": "fresh\n"},
    ])
    assert out == [
        {"path": "a.py", "old": "old\n", "new": "new\n"},
        {"path": "b.py", "old": "", "new": "fresh\n"},
    ]
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd orchestrator && uv run pytest tests/test_ui.py tests/test_apply.py -v`
Expected: FAIL — modules/functions missing.

- [ ] **Step 3: Implement**

`apply.py` — add:

```python
def changes_with_old(project_root: str, changes: list[FileChange]) -> list[dict]:
    """Pair each pending change with the file's current content for diff UIs."""
    out = []
    for change in changes:
        target = _resolve(project_root, change["path"])
        old = target.read_text() if target.exists() else ""
        out.append({"path": change["path"], "old": old, "new": change["content"]})
    return out
```

`ui.py`:

```python
"""UI channels for the orchestrator CLI.

JsonUI speaks the line-delimited JSON protocol consumed by the Go TUI.
PlainUI keeps the legacy human-readable terminal behaviour.
"""
import json
import sys

from mac_orchestrator.apply import render_diff


class JsonUI:
    def emit(self, **event) -> None:
        sys.stdout.write(json.dumps(event, ensure_ascii=False) + "\n")
        sys.stdout.flush()

    def read_command(self) -> dict:
        line = sys.stdin.readline()
        if not line:
            return {"cmd": "quit"}
        try:
            cmd = json.loads(line)
        except json.JSONDecodeError:
            return {"cmd": "quit"}
        if not isinstance(cmd, dict) or "cmd" not in cmd:
            return {"cmd": "quit"}
        return cmd


class PlainUI:
    def __init__(self) -> None:
        self._changes: list[dict] = []

    def emit(self, **event) -> None:
        kind = event.get("event")
        if kind == "session":
            print(f"Project: {event.get('project')}\nSession: {event.get('session_id')}\n"
                  f"Task:    {event.get('task')}\n")
        elif kind == "phase":
            print(f"  [{event.get('phase')}] iteration {event.get('iterations')} "
                  f"{event.get('verdict', '')}")
        elif kind == "test_output":
            print(f"  tests exit={event.get('exit_code')}")
        elif kind == "await_approval":
            self._changes = event.get("changes", [])
            print("\n" + "=" * 60)
            for c in self._changes:
                print(f"--- {c['path']} ---")
            print("=" * 60)
        elif kind == "done":
            print(f"\nDone — tests green after {event.get('iterations', 0)} test runs.")
        elif kind == "halt":
            print(f"\nStopped: {event.get('reason')}", file=sys.stderr)
        elif kind == "error":
            print(f"error: {event.get('message')}", file=sys.stderr)
        # token / node_end are silent in plain mode

    def read_command(self) -> dict:
        choice = input("\n[a]pprove & apply  [f]eedback  [q]uit > ").strip().lower()
        if choice.startswith("q"):
            return {"cmd": "quit"}
        if choice.startswith("f"):
            return {"cmd": "feedback", "text": input("feedback > ").strip()}
        return {"cmd": "approve"}
```

(Plain mode loses the inline unified diff body in this refactor unless re-added: in `__main__.py` plain path, print `render_diff` before `read_command` — see below, the diff is rendered from the structured changes via a small helper so behaviour is preserved.)

`__main__.py` — rework `main`/`run`:

```python
"""mac-orchestrator CLI: drive the TDD graph with human-in-the-loop approval.

Invoked by the Go binary as:
  mac-orchestrator --project <abs path> --task "<task>" --db <dsn> --ui json
"""
import argparse
import os
import sys

from mac_orchestrator.apply import changes_with_old, render_diff
from mac_orchestrator.config import load_config
from mac_orchestrator.db import Database
from mac_orchestrator.embeddings import embed_or_none
from mac_orchestrator.graph import build_graph
from mac_orchestrator.llm import build_chat_model
from mac_orchestrator.ui import JsonUI, PlainUI

DEFAULT_DSN = "postgres://postgres:postgres@localhost:5432/mac_cli?sslmode=disable"


def main() -> int:
    parser = argparse.ArgumentParser(prog="mac-orchestrator")
    parser.add_argument("--project", required=True, help="absolute project root")
    parser.add_argument("--task", required=True, help="coding task prompt")
    parser.add_argument("--db", default=os.environ.get("MAC_DB_URL", DEFAULT_DSN))
    parser.add_argument("--ui", choices=("plain", "json"), default="plain")
    args = parser.parse_args()

    ui = JsonUI() if args.ui == "json" else PlainUI()
    cfg = load_config(args.project)
    db = Database(args.db)
    try:
        return run(db, cfg, args.project, args.task, ui)
    except Exception as exc:  # surface as protocol event, not a traceback
        ui.emit(event="error", message=str(exc))
        return 1
    finally:
        db.close()


def _text(content) -> str:
    """LangChain chunk content may be str or a list of blocks."""
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        return "".join(
            b.get("text", "") if isinstance(b, dict) else str(b) for b in content)
    return str(content)


def run(db: Database, cfg, project_root: str, task: str, ui) -> int:
    project = db.find_project(project_root)
    if project is None:
        ui.emit(event="error", message=f"no mac project registered at {project_root}")
        return 1
    project_id, name = project
    session_id = db.create_session(project_id, task)
    ui.emit(event="session", project=name, session_id=session_id, task=task)

    models = {"planner": build_chat_model(cfg.planner),
              "coder": build_chat_model(cfg.coder)}
    graph = build_graph(models, db, project_id, session_id, embedder=embed_or_none)
    config = {"configurable": {"thread_id": session_id}}

    state = {"project_root": project_root, "task": task,
             "session_id": session_id, "messages": [], "step_order": 0}
    while True:
        for mode, chunk in graph.stream(state, config,
                                        stream_mode=["updates", "messages"]):
            if mode == "messages":
                msg, meta = chunk
                text = _text(getattr(msg, "content", ""))
                if text:
                    ui.emit(event="token",
                            node=meta.get("langgraph_node", ""), text=text)
            else:
                for node, update in (chunk or {}).items():
                    if node == "__interrupt__" or not isinstance(update, dict):
                        continue
                    ui.emit(event="node_end", node=node,
                            step=update.get("step_order", 0))
                    if node == "test_runner":
                        ui.emit(event="phase",
                                phase=str(update.get("tdd_phase", "")),
                                verdict=update.get("verdict", ""),
                                iterations=update.get("iterations", 0))
                        ui.emit(event="test_output",
                                exit_code=update.get("last_exit_code", 0),
                                tail=update.get("last_test_output", "")[-2000:])
        state = None  # after first run, always resume from checkpoint

        snapshot = graph.get_state(config)
        if not snapshot.next:
            break

        changes = changes_with_old(
            project_root, snapshot.values.get("pending_changes", []))
        if isinstance(ui, PlainUI):
            print(render_diff(project_root,
                              snapshot.values.get("pending_changes", [])))
        ui.emit(event="await_approval", changes=changes)
        cmd = ui.read_command()

        if cmd["cmd"] == "quit":
            ui.emit(event="halt", reason="aborted by user — no changes applied")
            return 130
        if cmd["cmd"] == "feedback":
            graph.update_state(
                config,
                {"messages": [("user", f"Operator feedback on your proposed "
                                       f"changes (regenerate): {cmd.get('text', '')}")],
                 "pending_changes": []},
                as_node="planner",
            )
        # approve: plain resume continues into 'apply'

    final = graph.get_state(config).values
    if final.get("verdict") == "done":
        ui.emit(event="done", iterations=final.get("iterations", 0))
        return 0
    ui.emit(event="halt",
            reason=f"iteration cap reached (last exit code {final.get('last_exit_code')})")
    return 1


if __name__ == "__main__":
    sys.exit(main())
```

- [ ] **Step 4: Run tests**

Run: `cd orchestrator && uv run pytest -v`
Expected: full suite PASS (existing `__main__`-related tests, if any, may need the `ui` parameter threaded — fix call sites in tests using `PlainUI()`).

- [ ] **Step 5: Commit**

```bash
git add orchestrator/src/ orchestrator/tests/
git commit -m "feat(orchestrator): JSON-lines UI protocol with plain fallback"
```

---

### Task 10: Go `mac code` Bubble Tea TUI

**Files:**
- Create: `internal/tui/events.go`, `internal/tui/coderun.go`
- Modify: `cmd/mac/main.go` (codeCmd hands off to `tui.RunCode`)
- Test: `internal/tui/events_test.go`, `internal/tui/coderun_test.go`

- [ ] **Step 1: Add dependencies**

```bash
go get github.com/aymanbagabas/go-udiff@latest github.com/alecthomas/chroma/v2@latest
```

- [ ] **Step 2: Write the failing tests**

`internal/tui/events_test.go`:

```go
package tui

import (
	"encoding/json"
	"testing"
)

func TestDecodeAwaitApproval(t *testing.T) {
	line := `{"event":"await_approval","changes":[{"path":"a.py","old":"x","new":"y"}]}`
	var ev CodeEvent
	if err := json.Unmarshal([]byte(line), &ev); err != nil {
		t.Fatal(err)
	}
	if ev.Event != "await_approval" || len(ev.Changes) != 1 || ev.Changes[0].Path != "a.py" {
		t.Fatalf("decoded %+v", ev)
	}
}

func TestEncodeCommand(t *testing.T) {
	b, err := json.Marshal(CodeCommand{Cmd: "feedback", Text: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"cmd":"feedback","text":"hi"}` {
		t.Fatalf("got %s", b)
	}
}
```

`internal/tui/coderun_test.go`:

```go
package tui

import (
	"bytes"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func newTestRun() *codeRunModel {
	m := newCodeRunModel("demo task")
	var buf bytes.Buffer
	m.cmdWriter = &buf
	return m
}

func TestAwaitApprovalSwitchesMode(t *testing.T) {
	m := newTestRun()
	mm, _ := m.Update(eventMsg{CodeEvent{Event: "await_approval",
		Changes: []FileDelta{{Path: "a.py", Old: "", New: "pass\n"}}}})
	m = mm.(*codeRunModel)
	if m.mode != modeApproval {
		t.Fatalf("mode = %v", m.mode)
	}
	if !strings.Contains(m.View(), "a.py") {
		t.Fatal("approval view must list the file")
	}
}

func TestApproveKeySendsCommand(t *testing.T) {
	m := newTestRun()
	mm, _ := m.Update(eventMsg{CodeEvent{Event: "await_approval",
		Changes: []FileDelta{{Path: "a.py", New: "pass\n"}}}})
	m = mm.(*codeRunModel)
	buf := m.cmdWriter.(*bytes.Buffer)
	mm, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = mm.(*codeRunModel)
	if !strings.Contains(buf.String(), `"cmd":"approve"`) {
		t.Fatalf("stdin got %q", buf.String())
	}
	if m.mode != modeRunning {
		t.Fatalf("mode = %v", m.mode)
	}
}

func TestPhaseEventUpdatesTracker(t *testing.T) {
	m := newTestRun()
	mm, _ := m.Update(eventMsg{CodeEvent{Event: "phase", Phase: "GREEN", Iterations: 3}})
	m = mm.(*codeRunModel)
	if m.phase != "GREEN" || m.iterations != 3 {
		t.Fatalf("phase=%q iter=%d", m.phase, m.iterations)
	}
}

func TestDoneEventQuits(t *testing.T) {
	m := newTestRun()
	mm, cmd := m.Update(eventMsg{CodeEvent{Event: "done", Iterations: 4}})
	m = mm.(*codeRunModel)
	if !m.finished || cmd == nil {
		t.Fatal("done must finish the program")
	}
}

func TestRenderUnifiedDiffColors(t *testing.T) {
	out := renderUnifiedDiff("a.py", "x = 1\n", "x = 2\n", 80)
	if !strings.Contains(out, "x = 1") || !strings.Contains(out, "x = 2") {
		t.Fatalf("diff missing lines:\n%s", out)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/tui/ -run 'TestDecode|TestEncode|TestAwait|TestApprove|TestPhase|TestDone|TestRenderUnified' -v`
Expected: FAIL — types undefined.

- [ ] **Step 4: Implement `events.go`**

```go
package tui

// CodeEvent is one line of the orchestrator's JSON protocol (--ui json).
type CodeEvent struct {
	Event      string      `json:"event"`
	Project    string      `json:"project,omitempty"`
	SessionID  string      `json:"session_id,omitempty"`
	Task       string      `json:"task,omitempty"`
	Node       string      `json:"node,omitempty"`
	Step       int         `json:"step,omitempty"`
	Text       string      `json:"text,omitempty"`
	Phase      string      `json:"phase,omitempty"`
	Verdict    string      `json:"verdict,omitempty"`
	Iterations int         `json:"iterations,omitempty"`
	ExitCode   int         `json:"exit_code,omitempty"`
	Tail       string      `json:"tail,omitempty"`
	Changes    []FileDelta `json:"changes,omitempty"`
	Reason     string      `json:"reason,omitempty"`
	Message    string      `json:"message,omitempty"`
}

// FileDelta carries full old/new contents; the Go side renders the diff.
type FileDelta struct {
	Path string `json:"path"`
	Old  string `json:"old"`
	New  string `json:"new"`
}

// CodeCommand is sent to the orchestrator's stdin.
type CodeCommand struct {
	Cmd  string `json:"cmd"`
	Text string `json:"text,omitempty"`
}
```

- [ ] **Step 5: Implement `coderun.go`**

```go
package tui

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/alecthomas/chroma/v2/quick"
	"github.com/aymanbagabas/go-udiff"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type runMode int

const (
	modeRunning runMode = iota
	modeApproval
	modeFeedback
)

type eventMsg struct{ ev CodeEvent }
type procExitMsg struct{ err error }

const maxIterations = 12 // mirror of routing.MAX_ITERATIONS for display

type codeRunModel struct {
	task       string
	sessionID  string
	mode       runMode
	phase      string
	iterations int
	finished   bool
	exitErr    string

	activity viewport.Model
	logLines []string
	stream   strings.Builder // current token stream

	changes  []FileDelta
	fileIdx  int
	diffView viewport.Model
	feedback textarea.Model

	cmdWriter io.Writer // orchestrator stdin (a *bytes.Buffer in tests)
	width     int
	height    int
}

func newCodeRunModel(task string) *codeRunModel {
	fb := textarea.New()
	fb.Placeholder = "what should change?"
	m := &codeRunModel{
		task:     task,
		activity: viewport.New(76, 10),
		diffView: viewport.New(76, 12),
		feedback: fb,
		width:    80, height: 24,
	}
	return m
}

func (m *codeRunModel) Init() tea.Cmd { return nil }

func (m *codeRunModel) send(c CodeCommand) {
	if m.cmdWriter == nil {
		return
	}
	b, _ := json.Marshal(c)
	fmt.Fprintln(m.cmdWriter, string(b))
}

func (m *codeRunModel) log(line string) {
	m.logLines = append(m.logLines, line)
	if len(m.logLines) > 200 {
		m.logLines = m.logLines[len(m.logLines)-200:]
	}
	m.activity.SetContent(strings.Join(m.logLines, "\n"))
	m.activity.GotoBottom()
}

func (m *codeRunModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.activity.Width = msg.Width - 4
		m.diffView.Width = msg.Width - 30
		return m, nil

	case eventMsg:
		return m.handleEvent(msg.ev)

	case procExitMsg:
		m.finished = true
		if msg.err != nil && m.exitErr == "" {
			m.exitErr = msg.err.Error()
		}
		return m, tea.Quit

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *codeRunModel) handleEvent(ev CodeEvent) (tea.Model, tea.Cmd) {
	switch ev.Event {
	case "session":
		m.sessionID = ev.SessionID
	case "token":
		m.stream.WriteString(ev.Text)
		// show last partial line of the stream as live activity
		tail := m.stream.String()
		if i := strings.LastIndexByte(tail, '\n'); i >= 0 {
			tail = tail[i+1:]
		}
		if len(tail) > 60 {
			tail = tail[len(tail)-60:]
		}
		m.replaceOrAppendStreamLine(fmt.Sprintf("▸ %s streaming… %q", ev.Node, tail))
	case "node_end":
		m.stream.Reset()
		m.log(fmt.Sprintf("▸ %s done (step %d)", ev.Node, ev.Step))
	case "phase":
		m.phase = ev.Phase
		m.iterations = ev.Iterations
		m.log(fmt.Sprintf("▸ phase %s · verdict %s", ev.Phase, ev.Verdict))
	case "test_output":
		status := SuccessStyle("PASS")
		if ev.ExitCode != 0 {
			status = ErrorStyle(fmt.Sprintf("FAIL (exit %d)", ev.ExitCode))
		}
		m.log("▸ tests: " + status)
	case "await_approval":
		m.changes = ev.Changes
		m.fileIdx = 0
		m.mode = modeApproval
		m.refreshDiff()
	case "done":
		m.log(SuccessStyle(fmt.Sprintf("✓ done — green after %d test runs", ev.Iterations)))
		m.finished = true
		return m, tea.Quit
	case "halt":
		m.exitErr = ev.Reason
		m.finished = true
		return m, tea.Quit
	case "error":
		m.exitErr = ev.Message
		m.finished = true
		return m, tea.Quit
	}
	return m, nil
}

// replaceOrAppendStreamLine keeps exactly one live "streaming…" line at the tail.
func (m *codeRunModel) replaceOrAppendStreamLine(line string) {
	if n := len(m.logLines); n > 0 && strings.Contains(m.logLines[n-1], "streaming…") {
		m.logLines[n-1] = line
	} else {
		m.logLines = append(m.logLines, line)
	}
	m.activity.SetContent(strings.Join(m.logLines, "\n"))
	m.activity.GotoBottom()
}

func (m *codeRunModel) handleKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.mode == modeFeedback {
		switch key.Type {
		case tea.KeyEsc:
			m.mode = modeApproval
			return m, nil
		case tea.KeyCtrlD: // submit feedback
			m.send(CodeCommand{Cmd: "feedback", Text: m.feedback.Value()})
			m.feedback.Reset()
			m.mode = modeRunning
			return m, nil
		}
		var cmd tea.Cmd
		m.feedback, cmd = m.feedback.Update(key)
		return m, cmd
	}

	switch key.String() {
	case "ctrl+c", "q":
		m.send(CodeCommand{Cmd: "quit"})
		m.finished = true
		return m, tea.Quit
	}

	if m.mode == modeApproval {
		switch key.String() {
		case "a":
			m.send(CodeCommand{Cmd: "approve"})
			m.mode = modeRunning
			m.log("▸ approved — applying")
		case "f":
			m.mode = modeFeedback
			m.feedback.Focus()
		case "tab":
			if len(m.changes) > 0 {
				m.fileIdx = (m.fileIdx + 1) % len(m.changes)
				m.refreshDiff()
			}
		case "j", "down":
			m.diffView.ScrollDown(1)
		case "k", "up":
			m.diffView.ScrollUp(1)
		}
	}
	return m, nil
}

func (m *codeRunModel) refreshDiff() {
	if m.fileIdx >= len(m.changes) {
		return
	}
	c := m.changes[m.fileIdx]
	m.diffView.SetContent(renderUnifiedDiff(c.Path, c.Old, c.New, m.diffView.Width))
	m.diffView.GotoTop()
}

// renderUnifiedDiff computes a unified diff and colorizes it: additions
// green (chroma-highlighted by file type), deletions red, hunks cyan.
func renderUnifiedDiff(path, old, new string, width int) string {
	unified := udiff.Unified("a/"+path, "b/"+path, old, new)
	var b strings.Builder
	for _, line := range strings.Split(unified, "\n") {
		switch {
		case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"):
			b.WriteString(DimStyle(line))
		case strings.HasPrefix(line, "@@"):
			b.WriteString(lipgloss.NewStyle().Foreground(clrCyan).Render(line))
		case strings.HasPrefix(line, "+"):
			b.WriteString(lipgloss.NewStyle().Foreground(clrGreen).Render("+") +
				highlightLine(path, strings.TrimPrefix(line, "+")))
		case strings.HasPrefix(line, "-"):
			b.WriteString(lipgloss.NewStyle().Foreground(clrRed).Render(line))
		default:
			b.WriteString(DimStyle(line))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// highlightLine best-effort syntax-highlights one source line with chroma.
func highlightLine(path, line string) string {
	var out strings.Builder
	if err := quick.Highlight(&out, line, path, "terminal256", "monokai"); err != nil {
		return line
	}
	return strings.TrimSuffix(out.String(), "\n")
}

func (m *codeRunModel) View() string {
	var b strings.Builder
	b.WriteString(AccentStyle("◆ MAC · My Agentic CLI"))
	if m.sessionID != "" {
		b.WriteString(DimStyle("   session " + short(m.sessionID)))
	}
	b.WriteString("\n" + DimStyle("Task: "+m.task) + "\n\n")
	b.WriteString(renderPhaseTracker(m.phase, m.iterations) + "\n")

	box := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(clrBorder).Padding(0, 1)
	b.WriteString(box.Render(m.activity.View()) + "\n")

	switch m.mode {
	case modeApproval:
		b.WriteString(m.viewApproval())
	case modeFeedback:
		b.WriteString("\n" + AccentStyle("feedback (ctrl+d send · esc cancel)") + "\n")
		b.WriteString(m.feedback.View() + "\n")
	default:
		b.WriteString("\n" + HintStyle("q quit") + "\n")
	}
	return b.String()
}

func (m *codeRunModel) viewApproval() string {
	var files strings.Builder
	for i, c := range m.changes {
		marker := "  "
		style := DimStyle
		if i == m.fileIdx {
			marker = "▸ "
			style = SelectedStyle
		}
		files.WriteString(style(marker+c.Path) + "\n")
	}
	left := lipgloss.NewStyle().Width(26).Render(files.String())
	right := m.diffView.View()
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	help := HintStyle("a approve · f feedback · tab file · j/k scroll · q quit")
	return fmt.Sprintf("\n%s %d files\n%s\n%s\n",
		AccentStyle("── approval ─"), len(m.changes), body, help)
}

func renderPhaseTracker(phase string, iterations int) string {
	render := func(name string, clr lipgloss.Color) string {
		dot := "○"
		style := lipgloss.NewStyle().Foreground(clrMuted)
		if phase == name {
			dot = "●"
			style = lipgloss.NewStyle().Bold(true).Foreground(clr)
		}
		return style.Render(dot + " " + name)
	}
	return fmt.Sprintf(" %s ──── %s ──── %s        iteration %d/%d",
		render("RED", clrRed), render("GREEN", clrGreen),
		render("REFACTOR", clrBlue), iterations, maxIterations)
}

func short(s string) string {
	if len(s) > 8 {
		return s[:8] + "…"
	}
	return s
}

// RunCodeOpts configures the orchestrator subprocess.
type RunCodeOpts struct {
	Bin     string // orchestrator binary (default mac-orchestrator)
	Project string
	Task    string
	DSN     string
}

// RunCode launches the orchestrator with --ui json and drives the TUI.
func RunCode(ctx context.Context, opts RunCodeOpts) error {
	cmd := exec.CommandContext(ctx, opts.Bin,
		"--project", opts.Project, "--task", opts.Task,
		"--db", opts.DSN, "--ui", "json")
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start orchestrator: %w\n\nInstall it with: uv tool install --from ./orchestrator mac-orchestrator", err)
	}

	model := newCodeRunModel(opts.Task)
	model.cmdWriter = stdin
	prog := tea.NewProgram(model)

	go func() {
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		for scanner.Scan() {
			var ev CodeEvent
			if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
				continue // tolerate stray non-JSON lines
			}
			prog.Send(eventMsg{ev})
		}
		prog.Send(procExitMsg{err: cmd.Wait()})
	}()

	out, err := prog.Run()
	if err != nil {
		return err
	}
	final := out.(*codeRunModel)
	if final.exitErr != "" {
		return fmt.Errorf("%s", final.exitErr)
	}
	return nil
}
```

- [ ] **Step 6: Wire `cmd/mac/main.go`**

Replace the body of `codeCmd`'s `RunE` after the project check with:

```go
			bin := os.Getenv("MAC_ORCHESTRATOR")
			if bin == "" {
				bin = "mac-orchestrator"
			}
			return tui.RunCode(ctx, tui.RunCodeOpts{
				Bin:     bin,
				Project: cwd,
				Task:    args[0],
				DSN:     resolveDSN(cmd),
			})
```

Remove the now-unused `os/exec` import from `main.go`.

- [ ] **Step 7: Run tests + build**

Run: `go build ./... && go test ./internal/tui/ -v && go mod tidy`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/tui/ cmd/mac/main.go go.mod go.sum
git commit -m "feat(tui): bubbletea code-run UI over JSON event protocol"
```

---

### Task 11: Animated MAC banner + concurrent DB connect

**Files:**
- Create: `internal/tui/banner.go`
- Modify: `cmd/mac/main.go` (`--no-banner` flag, banner before picker, concurrent connect)
- Test: `internal/tui/banner_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package tui

import (
	"strings"
	"testing"
)

func TestLerpHexEndpoints(t *testing.T) {
	if got := lerpHex("#7C3AED", "#3B82F6", 0); got != "#7C3AED" {
		t.Fatalf("t=0 got %s", got)
	}
	if got := lerpHex("#7C3AED", "#3B82F6", 1); got != "#3B82F6" {
		t.Fatalf("t=1 got %s", got)
	}
}

func TestBannerFrameContainsLogoAndTagline(t *testing.T) {
	frame := renderBannerFrame(1.0) // final frame: full gradient + full tagline
	if !strings.Contains(frame, "█") {
		t.Fatal("logo missing")
	}
	for _, ch := range []string{"M", "y", "A", "g", "e", "n", "t", "i", "c", "C", "L", "I"} {
		if !strings.Contains(frame, ch) {
			t.Fatalf("tagline incomplete, missing %q", ch)
		}
	}
}

func TestBannerFrameZeroShowsMACOnly(t *testing.T) {
	frame := renderBannerFrame(0)
	if strings.Contains(frame, "Agentic") {
		t.Fatal("tagline must not be expanded at t=0")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/ -run TestLerp -run TestBanner -v` (run both patterns separately)
Expected: FAIL.

- [ ] **Step 3: Implement `banner.go`**

```go
package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

const macLogo = `███╗   ███╗ █████╗  ██████╗
████╗ ████║██╔══██╗██╔════╝
██╔████╔██║███████║██║
██║╚██╔╝██║██╔══██║██║
██║ ╚═╝ ██║██║  ██║╚██████╗
╚═╝     ╚═╝╚═╝  ╚═╝ ╚═════╝`

const tagline = "My Agentic CLI"

// lerpHex linearly interpolates two #RRGGBB colors.
func lerpHex(a, b string, t float64) string {
	parse := func(s string) (int64, int64, int64) {
		var r, g, bl int64
		fmt.Sscanf(s, "#%02x%02x%02x", &r, &g, &bl)
		return r, g, bl
	}
	ar, ag, ab := parse(a)
	br, bg, bb := parse(b)
	mix := func(x, y int64) int64 { return x + int64(t*float64(y-x)) }
	return fmt.Sprintf("#%02X%02X%02X", mix(ar, br), mix(ag, bg), mix(ab, bb))
}

// renderBannerFrame renders the banner at animation progress t in [0,1].
// Gradient sweeps left→right with t; tagline typewriter-expands with t.
func renderBannerFrame(t float64) string {
	lines := strings.Split(macLogo, "\n")
	width := 0
	for _, l := range lines {
		if len([]rune(l)) > width {
			width = len([]rune(l))
		}
	}

	var b strings.Builder
	for _, line := range lines {
		runes := []rune(line)
		for i, r := range runes {
			pos := float64(i) / float64(width)
			// cells past the sweep front stay muted
			clr := string(clrBorder)
			if pos <= t {
				clr = lerpHex(string(clrPurple), string(clrBlue), pos)
			}
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(clr)).Render(string(r)))
		}
		b.WriteString("\n")
	}

	shown := int(t * float64(len(tagline)))
	if shown > len(tagline) {
		shown = len(tagline)
	}
	spaced := strings.Join(strings.Split(tagline[:shown], ""), " ")
	b.WriteString("        " + AccentStyle(spaced) + "\n")
	return b.String()
}

type bannerModel struct {
	t       float64
	done    bool
	skipped bool
}

type bannerTick struct{}

func tickBanner() tea.Cmd {
	return tea.Tick(40*time.Millisecond, func(time.Time) tea.Msg { return bannerTick{} })
}

func (m bannerModel) Init() tea.Cmd { return tickBanner() }

func (m bannerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case tea.KeyMsg:
		m.skipped = true
		return m, tea.Quit
	case bannerTick:
		m.t += 0.05 // ~20 frames * 40ms = 800ms total
		if m.t >= 1.0 {
			m.t = 1.0
			m.done = true
			return m, tea.Quit
		}
		return m, tickBanner()
	}
	return m, nil
}

func (m bannerModel) View() string { return "\n" + renderBannerFrame(m.t) }

// ShowBanner plays the animated banner. It is skipped when stdout is not a
// TTY, when MAC_NO_BANNER is set, or when noBanner is true. Always ≤ 1s;
// any key skips.
func ShowBanner(noBanner bool) {
	if noBanner || os.Getenv("MAC_NO_BANNER") != "" ||
		!term.IsTerminal(int(os.Stdout.Fd())) {
		return
	}
	p := tea.NewProgram(bannerModel{})
	if _, err := p.Run(); err != nil {
		return // banner is cosmetic; never fail the CLI over it
	}
	fmt.Print(renderBannerFrame(1.0)) // leave the final frame on screen
}

// CompactBrand is the one-line header used by subcommands.
func CompactBrand() string {
	return AccentStyle("◆ MAC") + DimStyle(" · "+tagline)
}
```

Check `golang.org/x/term` is in `go.mod` (transitively present via bubbletea; `go mod tidy` promotes it).

- [ ] **Step 4: Wire into `cmd/mac/main.go`**

Add persistent flag in `main()`:

```go
	root.PersistentFlags().Bool("no-banner", false, "skip the animated banner")
```

Rework `runRoot` to connect concurrently while the banner plays:

```go
func runRoot(cmd *cobra.Command, _ []string) error {
	ctx := context.Background()

	type connResult struct {
		db  *db.DB
		err error
	}
	ch := make(chan connResult, 1)
	go func() {
		d, err := connectDB(ctx, cmd)
		ch <- connResult{d, err}
	}()

	noBanner, _ := cmd.Flags().GetBool("no-banner")
	tui.ShowBanner(noBanner) // ≤1s; DB dial runs underneath

	res := <-ch
	if res.err != nil {
		return res.err
	}
	defer res.db.Close()
	return tui.Run(ctx, res.db)
}
```

In `codeCmd` `RunE`, print the compact brand before starting:

```go
			fmt.Println(tui.CompactBrand())
```

- [ ] **Step 5: Run tests + build**

Run: `go build ./... && go test ./... && go mod tidy`
Expected: PASS.

- [ ] **Step 6: Manual smoke test**

Run: `go run ./cmd/mac --help` (no banner — help path), then `MAC_NO_BANNER=1 go run ./cmd/mac` and `go run ./cmd/mac` in a terminal with Postgres up.
Expected: banner animates ≤1s, then picker appears; with MAC_NO_BANNER no animation.

- [ ] **Step 7: Commit**

```bash
git add internal/tui/banner.go internal/tui/banner_test.go cmd/mac/main.go go.mod go.sum
git commit -m "feat(tui): animated MAC banner with concurrent DB connect"
```

---

## Final verification

- [ ] `go build ./... && go vet ./... && go test ./...` — all green.
- [ ] `cd orchestrator && uv run pytest -v` — all green.
- [ ] Manual: `go run ./cmd/mac` → banner → wizard end-to-end with infra=no path and with full k8s+aws path into a temp dir; inspect generated tree for absent declined folders and accurate README/CONTEXT files.
- [ ] Manual: `mac code "<small task>"` against a scaffolded project with Ollama running — TUI shows phases, diff approval works, feedback regenerates, q aborts cleanly.
