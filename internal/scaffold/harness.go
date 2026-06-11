package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func writeHarness(root string, a Answers) error {
	files := map[string]string{
		"AGENTS.md":      agentsMD(a),
		"CONTEXT_MAP.md": contextMapMD(a),
		"CONTEXT.md":     contextMD(a),
		"README.md":      readmeMD(a),
	}

	for dir, content := range subContextFiles(a) {
		files[filepath.Join(dir, "CONTEXT.md")] = content
	}

	for rel, content := range files {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", rel, err)
		}
	}
	return nil
}

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

const agentsBodyMD = `
## Golden Principles (Karpathy Minimal Style)

- Think first, code second
- Simple over clever — smallest correct change
- Verify with commands, not confidence
- Escalate when judgment is required

## TDD Rules (Non-Negotiable)

- RED: Write a failing test first. Must fail via assertion — not syntax error.
- GREEN: Write minimum implementation to pass. No speculative generality.
- REFACTOR: SOLID principles, correct casing per language, mandatory function prologue.

## Language Casing

| Language            | Convention  |
|---------------------|-------------|
| Python / Rust / Go  | snake_case  |
| TypeScript / JS / Java | camelCase |

## Mandatory Function Prologue

Every non-trivial function must document:

- **WHY** — reason this function exists
- **SCOPE** — what it touches and what it does not
- **RESOLVING** — the problem being solved
- **THE ISSUE** — root cause or context
- **HOW IT SOLVED** — mechanism of solution
- **USAGE** — copy-paste runnable example
`

func agentsMD(a Answers) string {
	return fmt.Sprintf("# AGENTS.md — Global Guardrails\n\nStack: %s  |  Generated: %s\n"+agentsBodyMD,
		strings.Join(stackParts(a), " · "),
		time.Now().Format("2006-01-02"),
	)
}

func contextMapMD(a Answers) string {
	type row struct{ dir, purpose, ctx string }
	var rows []row
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

var backendRunCmds = map[string]string{
	"fastapi":    "uv run uvicorn src.adapters.api.main:app --reload",
	"express":    "pnpm dev",
	"gin":        "go run ./cmd/server",
	"axum":       "cargo run",
	"springboot": "mvn spring-boot:run",
}
var backendTestCmds = map[string]string{
	"fastapi":    "uv run pytest -v",
	"express":    "pnpm test",
	"gin":        "go test ./...",
	"axum":       "cargo test",
	"springboot": "mvn test",
}
var frontendRunCmds = map[string]string{
	"vanilla": "python -m http.server 8080",
	"react":   "pnpm dev",
	"nextjs":  "pnpm dev",
	"svelte":  "pnpm dev",
}

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
