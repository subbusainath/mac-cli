# Wizard Revamp & Code TUI Design

**Date:** 2026-06-11
**Status:** Approved
**Scope:** `mac` new-project wizard (optional infra, cloud gate, LLM API keys), conditional scaffold/docs generation, JSON event protocol between Go CLI and Python orchestrator, Bubble Tea TUI for `mac code`, branded animated banner.

**Decisions locked during brainstorming:**
- Python orchestrator stays (no Go rewrite).
- Infra: single-pick tiers — local / containers / containers+K8s.
- Cloud: yes/no gate, then single provider pick.
- API keys: global `~/.config/mac/credentials.toml` (0600), env vars take precedence, nothing key-like ever written into the project.
- LLM routing: user picks planner + coder providers from those with keys; local Ollama always available.
- `mac code` UI: Go Bubble Tea + JSON event protocol (not Python rich).
- Generated docs must reflect actual choices; no folders for declined options.

---

## 1. Wizard flow

```
 1. Name
 2. Path
 3. Backend?       yes/no → stack pick (fastapi/express/gin/axum/springboot)
 4. Frontend?      yes/no → stack pick (vanilla/react/nextjs/svelte)
 5. Infra?         yes/no
      yes → Deployment target: [local only | containers | containers + K8s]
      no  → skip 5a, 6, 7 entirely (no Docker, no K8s, no cloud, no IaC)
 6. Cloud deploy?  yes/no   (only reached when Infra = yes)
      no  → no cloud templates, no IaC step
 7. Cloud provider single pick (aws/azure/gcp) → IaC pick (existing per-cloud lists)
 8. LLM providers — for each of OpenAI / Claude (Anthropic) / DeepSeek / OpenRouter:
      key already found (env var or credentials.toml) → show ✓ badge, skip ask
      missing → "Have a <provider> API key?" yes/no → yes → masked paste input
      all four may be "no" → local Ollama fallback; wizard never blocks
 9. Planner + Coder provider picks — lists built from providers with keys
    + "local (Ollama)" always present. Defaults: planner = strongest with key
    (claude > openai > openrouter > deepseek > local); coder = cheapest with
    key (local > deepseek > openrouter > openai > claude).
    Model name pre-filled per provider, editable.
10. Confirm summary (infra tier, cloud-or-none, planner/coder providers+models)
```

Every gate honors "no" — nothing is forced, matching the existing
backend/frontend yes/no pattern. `esc` from a detail step returns to its gate.

## 2. Scaffold changes (Go)

- `scaffold.Answers` gains:
  - `Infra string` — `""` (declined) | `"local"` | `"containers"` | `"k8s"`
  - `Planner`, `Coder` — `{Provider, Model string}`
  - `Cloud`, `IAC` now allowed empty.
- `writeDockerfiles` runs only when Infra ∈ {containers, k8s}.
- New `internal/scaffold/k8s.go`: generates `k8s/` with `deployment.yaml`,
  `service.yaml`, `kustomization.yaml` per chosen backend/frontend — only when
  Infra = k8s.
- `writeCloudIaC` skipped when Cloud is empty.
- `config.Default` extended to write `[agents.planner]` / `[agents.coder]`
  with chosen provider + model into `.mac/config.toml`.

## 3. Credentials (new Go package `internal/credentials`)

- File: `~/.config/mac/credentials.toml`, written with mode 0600.
  Keys: `openai`, `anthropic`, `deepseek`, `openrouter`.
- Lookup order per provider: env var → credentials file. Env wins.
  Env vars: `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `DEEPSEEK_API_KEY`,
  `OPENROUTER_API_KEY`.
- Project `.mac/config.toml` stores provider/model names only — safe to commit.
- Wizard writes pasted keys here; never into the project tree.

## 4. Orchestrator changes (Python)

- `config.py`: accept new provider values (`openai`, `deepseek`, `openrouter`).
- `llm.py` provider map:
  - `anthropic` → `ChatAnthropic` (as today)
  - `openai`    → `ChatOpenAI` (api.openai.com)
  - `deepseek`  → `ChatOpenAI`, base `https://api.deepseek.com/v1`
  - `openrouter`→ `ChatOpenAI`, base `https://openrouter.ai/api/v1`
  - `local`     → `ChatOpenAI` against Ollama (as today)
- New `credentials.py`: same env-then-file lookup as Go side; injects the key
  into the model constructor. Missing key at runtime → clear error naming the
  env var and the wizard fix path.

## 5. Testing

- Go: wizard step-transition table tests (every gate's yes/no branch, esc
  behavior), credentials load/save + env precedence, k8s generator golden
  files, conditional-docs golden files (chosen vs declined combinations).
- Python: `llm.py` provider→constructor params, `credentials.py` lookup order,
  JSON event emitter / command reader round-trip. Existing conventions kept:
  no network or DB in unit tests.

## 6. Conditional docs + zero unwanted folders

**Folder rule: a directory exists only if its option was chosen.**

- `backend/`, `frontend/` — already conditional; stays.
- `infra/` only when Cloud chosen; `k8s/` only when Infra = k8s;
  Dockerfiles/compose only when Infra ≥ containers.
- `subContextFiles` becomes conditional — emit each `CONTEXT.md` only for
  chosen components (today it always writes backend/frontend/infra, creating
  declined folders with broken empty run commands).
- `.gitkeep` only in genuinely empty dirs; dropped where real files land.

**Docs templates become choice-aware (builders take full `Answers`):**

- `README.md` — quick start shows a real command (`docker compose up` only
  when containers; otherwise the per-stack dev command). Mermaid graph and
  directory tree render only chosen components. Stack line lists only chosen
  options plus planner/coder LLM choice.
- `CONTEXT.md` / `CONTEXT_MAP.md` — table rows and stack entries only for
  chosen dirs; no empty `/ /` slots.
- `AGENTS.md` — stack header from chosen parts only.
- All generated docs carry the new answers (infra tier, cloud-or-none, agent
  providers/models) so docs always match `.mac/config.toml` — one `Answers`
  value is the single source for both.

## 7. TUI

### 7a. Event protocol (orchestrator → Go)

Orchestrator gains `--ui json`: one JSON object per line on stdout; commands
read as JSON lines on stdin. Plain-text mode stays (`--ui plain`) for
debugging and future CI use.

```jsonl
{"event":"session","project":"...","session_id":"...","task":"..."}
{"event":"node_start","node":"planner","step":1}
{"event":"token","node":"coder","text":"..."}
{"event":"node_end","node":"coder","step":2}
{"event":"phase","phase":"GREEN","verdict":"coder","iterations":3}
{"event":"test_output","exit_code":1,"tail":"..."}
{"event":"await_approval","changes":[{"path":"...","old":"...","new":"..."}]}
{"event":"done","iterations":4}
{"event":"halt","reason":"..."}
{"event":"error","message":"..."}
```

Commands: `{"cmd":"approve"}` · `{"cmd":"feedback","text":"..."}` ·
`{"cmd":"quit"}`.

Raw `old`/`new` file contents are sent (not pre-rendered diffs) so the Go side
owns highlighting. This protocol is also the future daemon-mode wire format.

### 7b. `mac code` Bubble Tea UI (new `internal/tui/coderun.go`)

```
◆ MAC · My Agentic CLI                          session 8f3a…
Task: add rate limiter to login endpoint

 ● RED ──── ● GREEN ──── ○ REFACTOR        iteration 3/12
┌─ activity ────────────────────────────────────────────────┐
│ ▸ planner done (1.2s)                                      │
│ ▸ coder streaming… "def test_rate_limit_blocks_after…"     │
│ ▸ tests: FAILED 1 — AssertionError (tail below)            │
└────────────────────────────────────────────────────────────┘
── approval ─ 2 files ──────────────────────────────────────
 src/auth/limiter.py     │  @@ -0,0 +1,24 @@
 tests/test_limiter.py   │  +class RateLimiter: …
 a approve · f feedback · j/k scroll · tab file · q quit
```

- Phase tracker uses `theme.go` colors (RED red, GREEN green, REFACTOR blue),
  iteration counter against MAX_ITERATIONS.
- Activity viewport: node events, streamed tokens, test-output tail.
- Approval view: file list left, chroma-highlighted unified diff right,
  `tab` cycles files, `j/k` scrolls.
- `f` opens inline textarea for feedback → regenerate.
- v1 approval is all-or-nothing (matches graph semantics — `pending_changes`
  is one checkpoint unit). Per-file partial approve is a later extension.

### 7c. Wizard additions

- API-key step: masked input (`EchoPassword`); `✓ found in env` /
  `✓ saved in credentials` badges for detected providers, which skip ahead.
- Planner/coder pick: provider list shows default model in description,
  editable model field after pick.
- Confirm card extended: infra tier, cloud-or-none, planner/coder lines.
- Spinner components shared between wizard scaffold step and code run.

### 7d. Branding & animation

Animated brand header on TTY invocations:

```
███╗   ███╗ █████╗  ██████╗
████╗ ████║██╔══██╗██╔════╝
██╔████╔██║███████║██║
██║╚██╔╝██║██╔══██║██║
██║ ╚═╝ ██║██║  ██║╚██████╗
╚═╝     ╚═╝╚═╝  ╚═╝ ╚═════╝
        M y   A g e n t i c   C L I
```

- Gradient sweep: purple→blue (theme.go palette) across the logo via lipgloss
  color interpolation, ~500ms.
- Expansion reveal: `M A C` typewriter-expands to `My Agentic CLI`, ~400ms.
- Implementation: Bubble Tea intro model (`internal/tui/banner.go`),
  tick-driven, hands off to picker/wizard/coderun in-process.
- Speed rules (non-negotiable): total ≤ 1s; any keypress skips; auto-skipped
  when stdout is not a TTY; `--no-banner` flag and `MAC_NO_BANNER` env;
  DB connect runs concurrently during the animation so it costs zero
  wall-clock time.
- Compact variant for `mac code`: single-line `◆ MAC · My Agentic CLI`
  gradient header; full logo only on bare `mac` launch.
