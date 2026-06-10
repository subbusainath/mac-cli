package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
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

func agentsMD(a Answers) string {
	return fmt.Sprintf(`# AGENTS.md — Global Guardrails

Stack: %s / %s / %s / %s  |  Generated: %s

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
`,
		a.Backend, a.Frontend, a.Cloud, a.IAC,
		time.Now().Format("2006-01-02"),
	)
}

func contextMapMD(a Answers) string {
	return fmt.Sprintf(`# CONTEXT_MAP.md — Navigation Index

| Directory | Purpose                              | Context file         |
|-----------|--------------------------------------|----------------------|
| backend/  | %s service — Hexagonal Architecture  | backend/CONTEXT.md   |
| frontend/ | %s UI layer                          | frontend/CONTEXT.md  |
| infra/    | %s / %s infrastructure               | infra/CONTEXT.md     |
| docs/     | Architecture diagrams, ADRs          | —                    |
| scripts/  | Dev tooling, CI helpers              | —                    |
| .mac/     | mac CLI config (config.toml)         | —                    |
`, a.Backend, a.Frontend, a.Cloud, a.IAC)
}

func contextMD(a Answers) string {
	return fmt.Sprintf(`# CONTEXT.md — Root

## Truth Tier: Authoritative

## Stack

- Backend:  %s
- Frontend: %s
- Cloud:    %s / %s

## Run Commands

`+"```"+`bash
# Full stack
docker compose up

# Backend only  →  see backend/CONTEXT.md
# Frontend only →  see frontend/CONTEXT.md
`+"```"+`

## Active Plans

- [ ] Initial scaffold complete
- [ ] Add authentication
- [ ] Configure CI/CD pipeline
`, a.Backend, a.Frontend, a.Cloud, a.IAC)
}

func readmeMD(a Answers) string {
	return fmt.Sprintf(`# %s

> %s + %s on %s (%s)

## Quick Start

`+"```"+`bash
docker compose up
`+"```"+`

## Architecture

`+"```"+`mermaid
graph TB
    subgraph FE ["%s Frontend"]
        UI[UI Layer]
    end
    subgraph BE ["%s Backend — Hexagonal"]
        API[Adapters / API]
        APP[Application / Use Cases]
        DOM[Domain / Entities]
        PER[Adapters / Persistence]
    end
    subgraph INFRA ["%s / %s"]
        DB[(PostgreSQL + pgvector)]
    end

    UI  --> API
    API --> APP
    APP --> DOM
    APP --> PER
    PER --> DB
`+"```"+`

## Directory Layout

`+"```"+`
.
├── backend/   # %s  (Hexagonal Architecture)
├── frontend/  # %s
├── infra/     # %s / %s
├── docs/
└── .mac/      # mac CLI config
`+"```"+`
`,
		a.Name,
		a.Backend, a.Frontend, a.Cloud, a.IAC,
		a.Frontend, a.Backend, a.Cloud, a.IAC,
		a.Backend, a.Frontend, a.Cloud, a.IAC,
	)
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
	return map[string]string{
		"backend": fmt.Sprintf(`# CONTEXT.md — backend

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
`, backendRunCmds[a.Backend], backendTestCmds[a.Backend]),

		"frontend": fmt.Sprintf(`# CONTEXT.md — frontend

## Framework: %s

## Run

`+"```"+`bash
cd frontend && %s
`+"```"+`
`, a.Frontend, frontendRunCmds[a.Frontend]),

		"infra": `# CONTEXT.md — infra

## Contains IaC configuration only — no application logic.

## Apply

` + "```" + `bash
# See the subdirectory for your chosen tool (terraform / cdk / sam / pulumi / bicep)
` + "```" + `
`,
	}
}
