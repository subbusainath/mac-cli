# LangGraph TDD Orchestrator (Step 2) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Python LangGraph engine that `mac code "<task>"` hands off to, running a stateful RED→GREEN→REFACTOR TDD loop with PostgreSQL memory, pgvector knowledge lookup, error-feedback recall, dynamic LLM routing, and human-in-the-loop diff approval.

**Architecture:** A `StateGraph(MacAgentState)` with nodes planner → coder → apply → test_runner, plus a refactor node. `interrupt_before=["apply"]` pauses the graph so the CLI can show a diff and collect approve/feedback. The test_runner evaluates output per TDD phase and a conditional edge routes back to coder, on to refactor, or to END. All node I/O is recorded in `agent_tasks`; GREEN-phase failures are persisted to `terminal_errors_feedback` and matching historical fixes are piped back into the coder prompt.

**Tech Stack:** Python 3.12, uv, LangGraph, langchain-anthropic (planner), langchain-openai (local Ollama via OpenAI-compatible API + optional embeddings), psycopg 3, pytest. Go side: `os/exec` handoff in `cmd/mac/main.go`.

**Layout:** New top-level `orchestrator/` directory inside the mac-cli repo:

```
orchestrator/
  pyproject.toml
  src/mac_orchestrator/
    __init__.py
    __main__.py        # CLI entry + HIL loop
    state.py           # MacAgentState, TDDPhase, FileChange
    parsing.py         # parse_coder_output
    config.py          # .mac/config.toml loader
    db.py              # psycopg queries + pure helpers (error_hash, vector literal)
    llm.py             # provider -> chat model factory
    embeddings.py      # optional OpenAI embedder (1536 dims)
    shell.py           # subprocess test runner + output classifier
    routing.py         # evaluate() phase transitions + route_after_test
    apply.py           # diff rendering + safe writes
    nodes.py           # planner / coder / refactor / apply / test_runner factories
    graph.py           # StateGraph wiring + interrupt_before
  tests/
    test_parsing.py  test_config.py  test_db_helpers.py  test_shell.py
    test_routing.py  test_llm.py  test_apply.py  test_nodes.py  test_graph.py
```

Conventions: every node is a closure built by a `make_*(deps)` factory so tests inject fakes. Prompt builders are pure functions. No network or DB in unit tests; DB integration tests skip unless `MAC_TEST_DB` is set.

---

### Task 0: Commit existing Step 1 work

The repo has no commits yet. Lock in the Go scaffold first.

**Files:**
- Create: `.gitignore`

- [ ] **Step 1: Write .gitignore**

```gitignore
# Go
/mac
*.test

# Python (orchestrator)
__pycache__/
*.pyc
.venv/
orchestrator/.venv/
.pytest_cache/
*.egg-info/

# Local env
.env
```

- [ ] **Step 2: Verify Go build passes**

Run: `cd /Users/afreshstart/Projects/mac-cli && go build ./...`
Expected: exit 0, no output.

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "feat: Go CLI scaffold with PostgreSQL state layer (Step 1)"
```

---

### Task 1: Python project scaffold

**Files:**
- Create: `orchestrator/pyproject.toml`
- Create: `orchestrator/src/mac_orchestrator/__init__.py`
- Create: `orchestrator/tests/__init__.py`

- [ ] **Step 1: Write pyproject.toml**

```toml
[project]
name = "mac-orchestrator"
version = "0.1.0"
description = "LangGraph TDD orchestration engine for the mac CLI"
requires-python = ">=3.12"
dependencies = [
    "langgraph>=0.2.60",
    "langchain-core>=0.3.0",
    "langchain-anthropic>=0.3.0",
    "langchain-openai>=0.3.0",
    "psycopg[binary]>=3.2",
]

[project.scripts]
mac-orchestrator = "mac_orchestrator.__main__:main"

[dependency-groups]
dev = ["pytest>=8.0"]

[build-system]
requires = ["hatchling"]
build-backend = "hatchling.build"

[tool.hatch.build.targets.wheel]
packages = ["src/mac_orchestrator"]

[tool.pytest.ini_options]
testpaths = ["tests"]
```

- [ ] **Step 2: Create package skeleton**

```bash
mkdir -p orchestrator/src/mac_orchestrator orchestrator/tests
touch orchestrator/src/mac_orchestrator/__init__.py orchestrator/tests/__init__.py
```

- [ ] **Step 3: Sync and verify pytest runs**

Run: `cd orchestrator && uv sync && uv run pytest`
Expected: `no tests ran` (exit code 5 is fine at this point).

- [ ] **Step 4: Commit**

```bash
git add orchestrator/
git commit -m "feat(orchestrator): uv project scaffold"
```

---

### Task 2: State types + coder output parsing

**Files:**
- Create: `orchestrator/src/mac_orchestrator/state.py`
- Create: `orchestrator/src/mac_orchestrator/parsing.py`
- Test: `orchestrator/tests/test_parsing.py`

- [ ] **Step 1: Write state.py** (types only — no test needed)

```python
"""Shared state for the mac TDD graph."""
from enum import StrEnum
from typing import Annotated, TypedDict

from langchain_core.messages import AnyMessage
from langgraph.graph.message import add_messages


class TDDPhase(StrEnum):
    RED = "RED"
    GREEN = "GREEN"
    REFACTOR = "REFACTOR"


class FileChange(TypedDict):
    path: str       # relative to project root
    content: str    # full new file content


class MacAgentState(TypedDict, total=False):
    project_root: str
    task: str
    session_id: str
    messages: Annotated[list[AnyMessage], add_messages]
    plan: str
    tdd_phase: str                  # TDDPhase value
    pending_changes: list[FileChange]
    test_command: str
    last_test_output: str
    last_exit_code: int
    open_error_hash: str            # GREEN failure awaiting a fix
    iterations: int
    step_order: int
    verdict: str                    # routing decision from test_runner
```

- [ ] **Step 2: Write failing tests for parsing**

```python
# orchestrator/tests/test_parsing.py
import pytest

from mac_orchestrator.parsing import parse_coder_output

REPLY = '''Here is the change.

```json
{
  "changes": [
    {"path": "tests/test_auth.py", "content": "def test_x():\\n    assert add(1, 1) == 2\\n"}
  ],
  "test_command": "uv run pytest tests/test_auth.py -v"
}
```
'''


def test_parses_changes_and_command():
    changes, cmd = parse_coder_output(REPLY)
    assert changes == [
        {"path": "tests/test_auth.py", "content": "def test_x():\n    assert add(1, 1) == 2\n"}
    ]
    assert cmd == "uv run pytest tests/test_auth.py -v"


def test_missing_json_block_raises():
    with pytest.raises(ValueError, match="json"):
        parse_coder_output("no block here")


def test_missing_test_command_defaults_empty():
    reply = '```json\n{"changes": []}\n```'
    changes, cmd = parse_coder_output(reply)
    assert changes == []
    assert cmd == ""
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd orchestrator && uv run pytest tests/test_parsing.py -v`
Expected: FAIL — `ModuleNotFoundError: No module named 'mac_orchestrator.parsing'`

- [ ] **Step 4: Implement parsing.py**

```python
"""Extract structured file changes from coder LLM replies."""
import json
import re

from mac_orchestrator.state import FileChange

_JSON_BLOCK = re.compile(r"```json\s*(\{.*?\})\s*```", re.DOTALL)


def parse_coder_output(text: str) -> tuple[list[FileChange], str]:
    match = _JSON_BLOCK.search(text)
    if match is None:
        raise ValueError("coder reply has no ```json block with changes")
    data = json.loads(match.group(1))
    changes: list[FileChange] = [
        {"path": c["path"], "content": c["content"]} for c in data.get("changes", [])
    ]
    return changes, data.get("test_command", "")
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd orchestrator && uv run pytest tests/test_parsing.py -v`
Expected: 3 passed.

- [ ] **Step 6: Commit**

```bash
git add orchestrator/src/mac_orchestrator/state.py orchestrator/src/mac_orchestrator/parsing.py orchestrator/tests/test_parsing.py
git commit -m "feat(orchestrator): agent state types and coder output parsing"
```

---

### Task 3: Config loader

**Files:**
- Create: `orchestrator/src/mac_orchestrator/config.py`
- Test: `orchestrator/tests/test_config.py`

- [ ] **Step 1: Write failing tests**

```python
# orchestrator/tests/test_config.py
from pathlib import Path

from mac_orchestrator.config import load_config

SAMPLE = """
[project]
name = "demo"
backend = "fastapi"
frontend = "nextjs"
cloud = "aws"
iac = "terraform"

[agents.planner]
provider = "anthropic"
model = "claude-sonnet-4-6"

[agents.coder]
provider = "local"
model = "qwen2.5-coder:14b"
api_base = "http://localhost:11434"
"""


def write_cfg(root: Path, body: str) -> None:
    (root / ".mac").mkdir()
    (root / ".mac" / "config.toml").write_text(body)


def test_loads_full_config(tmp_path):
    write_cfg(tmp_path, SAMPLE)
    cfg = load_config(tmp_path)
    assert cfg.project_name == "demo"
    assert cfg.backend == "fastapi"
    assert cfg.planner.provider == "anthropic"
    assert cfg.coder.model == "qwen2.5-coder:14b"
    assert cfg.coder.api_base == "http://localhost:11434"


def test_missing_agents_fall_back_to_defaults(tmp_path):
    write_cfg(tmp_path, '[project]\nname = "x"\n')
    cfg = load_config(tmp_path)
    assert cfg.planner.provider == "anthropic"
    assert cfg.coder.provider == "local"
    assert cfg.coder.api_base is None
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd orchestrator && uv run pytest tests/test_config.py -v`
Expected: FAIL — `ModuleNotFoundError`

- [ ] **Step 3: Implement config.py**

```python
"""Read the project's .mac/config.toml written by the Go CLI."""
import tomllib
from dataclasses import dataclass
from pathlib import Path


@dataclass(frozen=True)
class AgentLLM:
    provider: str
    model: str
    api_base: str | None = None


@dataclass(frozen=True)
class MacConfig:
    project_name: str
    backend: str
    frontend: str
    planner: AgentLLM
    coder: AgentLLM


def load_config(project_root: str | Path) -> MacConfig:
    path = Path(project_root) / ".mac" / "config.toml"
    raw = tomllib.loads(path.read_text())
    project = raw.get("project", {})
    agents = raw.get("agents", {})

    def agent(name: str, provider: str, model: str) -> AgentLLM:
        section = agents.get(name, {})
        return AgentLLM(
            provider=section.get("provider", provider),
            model=section.get("model", model),
            api_base=section.get("api_base"),
        )

    return MacConfig(
        project_name=project.get("name", ""),
        backend=project.get("backend", ""),
        frontend=project.get("frontend", ""),
        planner=agent("planner", "anthropic", "claude-sonnet-4-6"),
        coder=agent("coder", "local", "qwen2.5-coder:14b"),
    )
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd orchestrator && uv run pytest tests/test_config.py -v`
Expected: 2 passed.

- [ ] **Step 5: Commit**

```bash
git add orchestrator/src/mac_orchestrator/config.py orchestrator/tests/test_config.py
git commit -m "feat(orchestrator): .mac/config.toml loader"
```

---

### Task 4: Database layer

Pure helpers (hashing, normalization, vector literals) get unit tests. The `Database` class is thin SQL; integration tests run only when `MAC_TEST_DB` is set.

**Files:**
- Create: `orchestrator/src/mac_orchestrator/db.py`
- Test: `orchestrator/tests/test_db_helpers.py`

- [ ] **Step 1: Write failing tests for the pure helpers**

```python
# orchestrator/tests/test_db_helpers.py
from mac_orchestrator.db import error_hash, normalize_error, to_vector_literal


def test_normalize_strips_volatile_parts():
    a = normalize_error('File "/tmp/x/app.py", line 42, in handler\n  ValueError: bad input 0x7f3a')
    b = normalize_error('File "/tmp/y/app.py", line 99, in handler\n  ValueError: bad input 0x55ee')
    assert a == b


def test_error_hash_stable_and_hex():
    h1 = error_hash("ValueError: boom")
    h2 = error_hash("ValueError: boom")
    assert h1 == h2
    assert len(h1) == 64
    assert int(h1, 16)  # valid hex


def test_vector_literal_format():
    assert to_vector_literal([0.1, -1.0, 2.5]) == "[0.1,-1.0,2.5]"
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd orchestrator && uv run pytest tests/test_db_helpers.py -v`
Expected: FAIL — `ModuleNotFoundError`

- [ ] **Step 3: Implement db.py**

```python
"""PostgreSQL persistence: sessions, agent task log, error feedback, knowledge base."""
import hashlib
import json
import re
from typing import Any

import psycopg

# Volatile fragments that would break error-signature matching across runs.
_LINE_NO = re.compile(r"line \d+")
_HEX_ADDR = re.compile(r"0x[0-9a-fA-F]+")
_FILE_PATH = re.compile(r'File "[^"]+"')


def normalize_error(text: str) -> str:
    text = _FILE_PATH.sub('File "<path>"', text)
    text = _LINE_NO.sub("line <n>", text)
    text = _HEX_ADDR.sub("<addr>", text)
    return text.strip()


def error_hash(text: str) -> str:
    return hashlib.sha256(normalize_error(text).encode()).hexdigest()


def to_vector_literal(embedding: list[float]) -> str:
    return "[" + ",".join(repr(x) for x in embedding) + "]"


class Database:
    def __init__(self, dsn: str):
        self.conn = psycopg.connect(dsn, autocommit=True)

    def close(self) -> None:
        self.conn.close()

    def find_project(self, path: str) -> tuple[str, str] | None:
        row = self.conn.execute(
            "SELECT id, name FROM projects WHERE path = %s", (path,)
        ).fetchone()
        return (str(row[0]), row[1]) if row else None

    def create_session(self, project_id: str, user_prompt: str) -> str:
        row = self.conn.execute(
            "INSERT INTO sessions (project_id, user_prompt) VALUES (%s, %s) RETURNING id",
            (project_id, user_prompt),
        ).fetchone()
        return str(row[0])

    def record_agent_task(
        self,
        session_id: str,
        step_order: int,
        node_name: str,
        input_payload: dict[str, Any],
        output_payload: dict[str, Any],
        prompt_tokens: int = 0,
        completion_tokens: int = 0,
    ) -> None:
        self.conn.execute(
            """INSERT INTO agent_tasks
               (session_id, step_order, node_name, input_payload, output_payload,
                prompt_tokens, completion_tokens)
               VALUES (%s, %s, %s, %s, %s, %s, %s)""",
            (session_id, step_order, node_name, json.dumps(input_payload),
             json.dumps(output_payload), prompt_tokens, completion_tokens),
        )

    def save_error(self, project_id: str, error_text: str, target_folder: str) -> str:
        """Record a GREEN-phase failure; returns its hash. Fix attached later."""
        h = error_hash(error_text)
        self.conn.execute(
            """INSERT INTO terminal_errors_feedback
               (project_id, error_hash, error_signature, successful_fix, target_folder)
               VALUES (%s, %s, %s, '', %s)
               ON CONFLICT (project_id, error_hash) DO NOTHING""",
            (project_id, h, normalize_error(error_text)[:4000], target_folder),
        )
        return h

    def attach_fix(self, project_id: str, h: str, fix: str) -> None:
        self.conn.execute(
            """UPDATE terminal_errors_feedback SET successful_fix = %s
               WHERE project_id = %s AND error_hash = %s""",
            (fix[:8000], project_id, h),
        )

    def lookup_fixes(self, project_id: str, error_text: str | None = None,
                     target_folder: str | None = None, limit: int = 5) -> list[dict[str, str]]:
        """Historical fixes: exact hash match first, then same-folder matches."""
        clauses, params = ["project_id = %s", "successful_fix <> ''"], [project_id]
        if error_text is not None:
            clauses.append("error_hash = %s")
            params.append(error_hash(error_text))
        elif target_folder is not None:
            clauses.append("target_folder = %s")
            params.append(target_folder)
        rows = self.conn.execute(
            f"""SELECT error_signature, successful_fix FROM terminal_errors_feedback
                WHERE {' AND '.join(clauses)} ORDER BY created_at DESC LIMIT %s""",
            (*params, limit),
        ).fetchall()
        return [{"error": r[0], "fix": r[1]} for r in rows]

    def search_knowledge(self, embedding: list[float], limit: int = 5) -> list[str]:
        rows = self.conn.execute(
            """SELECT content FROM technical_knowledge_base
               WHERE embedding IS NOT NULL
               ORDER BY embedding <=> %s::vector LIMIT %s""",
            (to_vector_literal(embedding), limit),
        ).fetchall()
        return [r[0] for r in rows]
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd orchestrator && uv run pytest tests/test_db_helpers.py -v`
Expected: 3 passed.

- [ ] **Step 5: Commit**

```bash
git add orchestrator/src/mac_orchestrator/db.py orchestrator/tests/test_db_helpers.py
git commit -m "feat(orchestrator): postgres layer with error-signature hashing and pgvector search"
```

---

### Task 5: Shell test runner + output classifier

**Files:**
- Create: `orchestrator/src/mac_orchestrator/shell.py`
- Test: `orchestrator/tests/test_shell.py`

- [ ] **Step 1: Write failing tests**

```python
# orchestrator/tests/test_shell.py
from mac_orchestrator.shell import classify_failure, run_test_command


def test_run_captures_exit_and_output(tmp_path):
    result = run_test_command("echo hello && exit 3", cwd=str(tmp_path))
    assert result.exit_code == 3
    assert "hello" in result.output


def test_run_success(tmp_path):
    result = run_test_command("true", cwd=str(tmp_path))
    assert result.exit_code == 0


def test_classify_clean_assertion_failure():
    out = "FAILED tests/test_x.py::test_x - AssertionError: assert 1 == 2"
    assert classify_failure(out) == "clean"


def test_classify_dirty_syntax_error():
    out = 'E   SyntaxError: invalid syntax\nERROR collecting tests/test_x.py'
    assert classify_failure(out) == "dirty"


def test_classify_dirty_missing_command():
    assert classify_failure("zsh: command not found: pytset") == "dirty"


def test_classify_unknown_failure_treated_clean():
    assert classify_failure("1 test failed: expected 200 got 500") == "clean"
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd orchestrator && uv run pytest tests/test_shell.py -v`
Expected: FAIL — `ModuleNotFoundError`

- [ ] **Step 3: Implement shell.py**

```python
"""Run project test/build commands and classify their failures."""
import subprocess
from dataclasses import dataclass

_MAX_OUTPUT = 8000  # keep prompts bounded; tail holds the stack trace

# Failures that mean the test never reached an assertion — broken syntax,
# missing modules, bad commands. RED phase must not accept these.
_DIRTY_MARKERS = (
    "SyntaxError", "IndentationError", "ImportError", "ModuleNotFoundError",
    "ERROR collecting", "command not found", "No such file or directory",
    "error[E", "cannot find module", "compilation failed", "undefined:",
)


@dataclass(frozen=True)
class TestResult:
    exit_code: int
    output: str


def run_test_command(command: str, cwd: str, timeout: int = 300) -> TestResult:
    try:
        proc = subprocess.run(
            command, shell=True, cwd=cwd,
            capture_output=True, text=True, timeout=timeout,
        )
        output = (proc.stdout + "\n" + proc.stderr).strip()
        return TestResult(proc.returncode, output[-_MAX_OUTPUT:])
    except subprocess.TimeoutExpired:
        return TestResult(124, f"TIMEOUT: command exceeded {timeout}s: {command}")


def classify_failure(output: str) -> str:
    """'dirty' = infrastructure/syntax error; 'clean' = genuine test failure."""
    if any(marker in output for marker in _DIRTY_MARKERS):
        return "dirty"
    return "clean"
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd orchestrator && uv run pytest tests/test_shell.py -v`
Expected: 6 passed.

- [ ] **Step 5: Commit**

```bash
git add orchestrator/src/mac_orchestrator/shell.py orchestrator/tests/test_shell.py
git commit -m "feat(orchestrator): subprocess test runner with failure classification"
```

---

### Task 6: TDD phase evaluation + routing

**Files:**
- Create: `orchestrator/src/mac_orchestrator/routing.py`
- Test: `orchestrator/tests/test_routing.py`

- [ ] **Step 1: Write failing tests**

```python
# orchestrator/tests/test_routing.py
from mac_orchestrator.routing import MAX_ITERATIONS, evaluate, route_after_test
from mac_orchestrator.state import TDDPhase


def test_red_passing_test_loops_back():
    phase, verdict, feedback = evaluate(TDDPhase.RED, exit_code=0, output="2 passed")
    assert phase == TDDPhase.RED
    assert verdict == "coder"
    assert "fail" in feedback.lower()


def test_red_dirty_failure_loops_back():
    phase, verdict, _ = evaluate(TDDPhase.RED, 1, "SyntaxError: invalid syntax")
    assert (phase, verdict) == (TDDPhase.RED, "coder")


def test_red_clean_failure_advances_to_green():
    phase, verdict, _ = evaluate(TDDPhase.RED, 1, "AssertionError: assert 1 == 2")
    assert (phase, verdict) == (TDDPhase.GREEN, "coder")


def test_green_failure_stays_green():
    phase, verdict, _ = evaluate(TDDPhase.GREEN, 1, "AssertionError")
    assert (phase, verdict) == (TDDPhase.GREEN, "coder")


def test_green_pass_advances_to_refactor():
    phase, verdict, _ = evaluate(TDDPhase.GREEN, 0, "3 passed")
    assert (phase, verdict) == (TDDPhase.REFACTOR, "refactor")


def test_refactor_pass_is_done():
    phase, verdict, _ = evaluate(TDDPhase.REFACTOR, 0, "3 passed")
    assert verdict == "done"


def test_refactor_breakage_goes_back_to_green():
    phase, verdict, _ = evaluate(TDDPhase.REFACTOR, 1, "AssertionError")
    assert (phase, verdict) == (TDDPhase.GREEN, "coder")


def test_route_reads_verdict():
    assert route_after_test({"verdict": "coder", "iterations": 1}) == "coder"
    assert route_after_test({"verdict": "done", "iterations": 1}) == "done"


def test_route_halts_at_iteration_cap():
    assert route_after_test({"verdict": "coder", "iterations": MAX_ITERATIONS}) == "halt"
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd orchestrator && uv run pytest tests/test_routing.py -v`
Expected: FAIL — `ModuleNotFoundError`

- [ ] **Step 3: Implement routing.py**

```python
"""TDD phase transitions and conditional-edge routing."""
from mac_orchestrator.shell import classify_failure
from mac_orchestrator.state import MacAgentState, TDDPhase

MAX_ITERATIONS = 12  # hard cap on coder<->test loops to prevent runaway sessions


def evaluate(phase: TDDPhase, exit_code: int, output: str) -> tuple[TDDPhase, str, str]:
    """Decide (next_phase, verdict, feedback) after a test run.

    Verdicts: 'coder' (loop back), 'refactor' (go refactor), 'done' (finish).
    """
    if phase == TDDPhase.RED:
        if exit_code == 0:
            return (TDDPhase.RED, "coder",
                    "The test PASSED immediately. A RED-phase test must fail via an "
                    "explicit assertion against the missing behaviour. Rewrite it so it "
                    "fails until the feature is implemented.")
        if classify_failure(output) == "dirty":
            return (TDDPhase.RED, "coder",
                    "The test errored before reaching an assertion (syntax/import/tooling "
                    f"problem), which does not count as a clean RED failure:\n{output}")
        return (TDDPhase.GREEN, "coder",
                "Test fails cleanly as expected. Now write the MINIMUM implementation "
                "code to make it pass. Do not modify the test.")

    if phase == TDDPhase.GREEN:
        if exit_code == 0:
            return (TDDPhase.REFACTOR, "refactor",
                    "All tests pass. Refactor the implementation now.")
        return (TDDPhase.GREEN, "coder", f"Tests still failing:\n{output}")

    # REFACTOR
    if exit_code == 0:
        return (TDDPhase.REFACTOR, "done", "")
    return (TDDPhase.GREEN, "coder",
            f"Refactor broke the tests — restore green:\n{output}")


def route_after_test(state: MacAgentState) -> str:
    if state.get("iterations", 0) >= MAX_ITERATIONS and state["verdict"] != "done":
        return "halt"
    return state["verdict"]
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd orchestrator && uv run pytest tests/test_routing.py -v`
Expected: 9 passed.

- [ ] **Step 5: Commit**

```bash
git add orchestrator/src/mac_orchestrator/routing.py orchestrator/tests/test_routing.py
git commit -m "feat(orchestrator): TDD phase evaluation and graph routing"
```

---

### Task 7: LLM factory + optional embeddings

**Files:**
- Create: `orchestrator/src/mac_orchestrator/llm.py`
- Create: `orchestrator/src/mac_orchestrator/embeddings.py`
- Test: `orchestrator/tests/test_llm.py`

- [ ] **Step 1: Write failing tests** (no network — construct only, inspect params)

```python
# orchestrator/tests/test_llm.py
import pytest
from langchain_anthropic import ChatAnthropic
from langchain_openai import ChatOpenAI

from mac_orchestrator.config import AgentLLM
from mac_orchestrator.llm import build_chat_model


def test_anthropic_provider(monkeypatch):
    monkeypatch.setenv("ANTHROPIC_API_KEY", "test-key")
    model = build_chat_model(AgentLLM("anthropic", "claude-sonnet-4-6"))
    assert isinstance(model, ChatAnthropic)
    assert model.model == "claude-sonnet-4-6"


def test_local_provider_points_at_ollama():
    model = build_chat_model(
        AgentLLM("local", "qwen2.5-coder:14b", api_base="http://localhost:11434")
    )
    assert isinstance(model, ChatOpenAI)
    assert model.model_name == "qwen2.5-coder:14b"
    assert "11434" in str(model.openai_api_base)


def test_local_provider_default_base():
    model = build_chat_model(AgentLLM("local", "qwen2.5-coder:14b"))
    assert "localhost:11434" in str(model.openai_api_base)


def test_unknown_provider_raises():
    with pytest.raises(ValueError, match="unknown provider"):
        build_chat_model(AgentLLM("watsonx", "x"))
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd orchestrator && uv run pytest tests/test_llm.py -v`
Expected: FAIL — `ModuleNotFoundError`

- [ ] **Step 3: Implement llm.py**

```python
"""Map .mac/config.toml agent entries to chat models.

'anthropic' -> Claude via ANTHROPIC_API_KEY.
'local'     -> any OpenAI-compatible server (Ollama) at api_base.
"""
from langchain_anthropic import ChatAnthropic
from langchain_openai import ChatOpenAI

from mac_orchestrator.config import AgentLLM

_DEFAULT_OLLAMA = "http://localhost:11434"


def build_chat_model(cfg: AgentLLM):
    if cfg.provider == "anthropic":
        return ChatAnthropic(model=cfg.model, max_tokens=8192)
    if cfg.provider == "local":
        base = (cfg.api_base or _DEFAULT_OLLAMA).rstrip("/")
        return ChatOpenAI(model=cfg.model, base_url=base + "/v1", api_key="ollama")
    raise ValueError(f"unknown provider: {cfg.provider}")
```

- [ ] **Step 4: Implement embeddings.py** (thin; covered by graph smoke test later)

```python
"""Optional task embeddings for pgvector knowledge lookup.

The knowledge base uses VECTOR(1536) = OpenAI text-embedding-3-small.
Without OPENAI_API_KEY the lookup is skipped gracefully.
"""
import os


def embed_or_none(text: str) -> list[float] | None:
    if not os.environ.get("OPENAI_API_KEY"):
        return None
    from langchain_openai import OpenAIEmbeddings

    embedder = OpenAIEmbeddings(model="text-embedding-3-small")
    return embedder.embed_query(text)
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd orchestrator && uv run pytest tests/test_llm.py -v`
Expected: 4 passed.

- [ ] **Step 6: Commit**

```bash
git add orchestrator/src/mac_orchestrator/llm.py orchestrator/src/mac_orchestrator/embeddings.py orchestrator/tests/test_llm.py
git commit -m "feat(orchestrator): provider-routed chat models and optional embeddings"
```

---

### Task 8: Diff rendering + safe file application

**Files:**
- Create: `orchestrator/src/mac_orchestrator/apply.py`
- Test: `orchestrator/tests/test_apply.py`

- [ ] **Step 1: Write failing tests**

```python
# orchestrator/tests/test_apply.py
import pytest

from mac_orchestrator.apply import apply_changes, render_diff


def test_apply_writes_new_file(tmp_path):
    changes = [{"path": "src/util.py", "content": "x = 1\n"}]
    written = apply_changes(str(tmp_path), changes)
    assert written == ["src/util.py"]
    assert (tmp_path / "src" / "util.py").read_text() == "x = 1\n"


def test_apply_overwrites_existing(tmp_path):
    (tmp_path / "a.py").write_text("old\n")
    apply_changes(str(tmp_path), [{"path": "a.py", "content": "new\n"}])
    assert (tmp_path / "a.py").read_text() == "new\n"


def test_apply_rejects_path_escape(tmp_path):
    with pytest.raises(ValueError, match="outside project root"):
        apply_changes(str(tmp_path), [{"path": "../evil.py", "content": ""}])


def test_diff_shows_old_and_new(tmp_path):
    (tmp_path / "a.py").write_text("old line\n")
    diff = render_diff(str(tmp_path), [{"path": "a.py", "content": "new line\n"}])
    assert "-old line" in diff
    assert "+new line" in diff


def test_diff_new_file_all_additions(tmp_path):
    diff = render_diff(str(tmp_path), [{"path": "b.py", "content": "added\n"}])
    assert "+added" in diff
    assert "(new file)" in diff
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd orchestrator && uv run pytest tests/test_apply.py -v`
Expected: FAIL — `ModuleNotFoundError`

- [ ] **Step 3: Implement apply.py**

```python
"""Render diffs for HIL review and write approved changes to disk."""
import difflib
from pathlib import Path

from mac_orchestrator.state import FileChange


def _resolve(project_root: str, rel_path: str) -> Path:
    root = Path(project_root).resolve()
    target = (root / rel_path).resolve()
    if not target.is_relative_to(root):
        raise ValueError(f"change path escapes outside project root: {rel_path}")
    return target


def render_diff(project_root: str, changes: list[FileChange]) -> str:
    blocks: list[str] = []
    for change in changes:
        target = _resolve(project_root, change["path"])
        if target.exists():
            old_lines = target.read_text().splitlines(keepends=True)
            header = change["path"]
        else:
            old_lines = []
            header = f"{change['path']} (new file)"
        diff = difflib.unified_diff(
            old_lines, change["content"].splitlines(keepends=True),
            fromfile=f"a/{change['path']}", tofile=f"b/{change['path']}",
        )
        blocks.append(f"--- {header} ---\n" + "".join(diff))
    return "\n\n".join(blocks)


def apply_changes(project_root: str, changes: list[FileChange]) -> list[str]:
    # Validate every path before writing anything — all-or-nothing.
    targets = [(_resolve(project_root, c["path"]), c) for c in changes]
    written: list[str] = []
    for target, change in targets:
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(change["content"])
        written.append(change["path"])
    return written
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd orchestrator && uv run pytest tests/test_apply.py -v`
Expected: 5 passed.

- [ ] **Step 5: Commit**

```bash
git add orchestrator/src/mac_orchestrator/apply.py orchestrator/tests/test_apply.py
git commit -m "feat(orchestrator): diff rendering and sandboxed file application"
```

---

### Task 9: Graph nodes

All nodes are factory-built closures over injected deps (`model`, `db`, `project_id`) so tests use fakes. Prompt builders are pure.

**Files:**
- Create: `orchestrator/src/mac_orchestrator/nodes.py`
- Test: `orchestrator/tests/test_nodes.py`

- [ ] **Step 1: Write failing tests**

```python
# orchestrator/tests/test_nodes.py
from langchain_core.messages import AIMessage

from mac_orchestrator.nodes import (
    build_coder_prompt,
    build_planner_prompt,
    make_coder,
    make_planner,
    make_test_runner,
)
from mac_orchestrator.state import TDDPhase


class FakeModel:
    def __init__(self, reply: str):
        self.reply = reply
        self.calls: list = []

    def invoke(self, messages):
        self.calls.append(messages)
        return AIMessage(content=self.reply)


class FakeDB:
    def __init__(self):
        self.tasks, self.errors, self.fixes = [], [], []

    def record_agent_task(self, *a, **k):
        self.tasks.append((a, k))

    def lookup_fixes(self, *a, **k):
        return self.fixes

    def save_error(self, project_id, error_text, target_folder):
        self.errors.append(error_text)
        return "deadbeef"

    def attach_fix(self, *a, **k):
        pass

    def search_knowledge(self, *a, **k):
        return []


CODER_REPLY = '''```json
{"changes": [{"path": "tests/test_add.py", "content": "def test_add():\\n    assert add(1,1) == 2\\n"}],
 "test_command": "uv run pytest tests/test_add.py -v"}
```'''


def test_planner_prompt_includes_context_sources():
    prompt = build_planner_prompt(
        task="add auth", context_map="| dir | ctx |",
        knowledge=["chunk-a"], fixes=[{"error": "E1", "fix": "F1"}],
    )
    for fragment in ("add auth", "| dir | ctx |", "chunk-a", "E1", "F1"):
        assert fragment in prompt


def test_planner_node_sets_plan_and_phase(tmp_path):
    model, db = FakeModel("1. write test\n2. implement"), FakeDB()
    planner = make_planner(model, db, project_id="p1", session_id="s1")
    out = planner({"project_root": str(tmp_path), "task": "add auth",
                   "messages": [], "step_order": 0})
    assert out["plan"] == "1. write test\n2. implement"
    assert out["tdd_phase"] == TDDPhase.RED
    assert len(db.tasks) == 1


def test_coder_prompt_varies_by_phase():
    red = build_coder_prompt({"tdd_phase": TDDPhase.RED, "task": "t", "plan": "p"})
    green = build_coder_prompt({"tdd_phase": TDDPhase.GREEN, "task": "t", "plan": "p"})
    assert "failing test" in red.lower()
    assert "minimum" in green.lower()


def test_coder_node_parses_changes(tmp_path):
    model, db = FakeModel(CODER_REPLY), FakeDB()
    coder = make_coder({"coder": model, "planner": model}, db,
                       project_id="p1", session_id="s1")
    out = coder({"project_root": str(tmp_path), "task": "t", "plan": "p",
                 "tdd_phase": TDDPhase.RED, "messages": [], "step_order": 1})
    assert out["pending_changes"][0]["path"] == "tests/test_add.py"
    assert out["test_command"].startswith("uv run pytest")


def test_test_runner_advances_red_to_green(tmp_path):
    db = FakeDB()
    runner = make_test_runner(db, project_id="p1", session_id="s1")
    out = runner({"project_root": str(tmp_path), "tdd_phase": TDDPhase.RED,
                  "test_command": "echo 'AssertionError: boom' && exit 1",
                  "messages": [], "iterations": 0, "step_order": 2,
                  "pending_changes": []})
    assert out["tdd_phase"] == TDDPhase.GREEN
    assert out["verdict"] == "coder"
    assert out["iterations"] == 1


def test_test_runner_saves_green_failure(tmp_path):
    db = FakeDB()
    runner = make_test_runner(db, project_id="p1", session_id="s1")
    out = runner({"project_root": str(tmp_path), "tdd_phase": TDDPhase.GREEN,
                  "test_command": "echo 'AssertionError: still broken' && exit 1",
                  "messages": [], "iterations": 1, "step_order": 3,
                  "pending_changes": [{"path": "src/x.py", "content": ""}]})
    assert db.errors  # failure persisted to terminal_errors_feedback
    assert out["open_error_hash"] == "deadbeef"
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd orchestrator && uv run pytest tests/test_nodes.py -v`
Expected: FAIL — `ModuleNotFoundError`

- [ ] **Step 3: Implement nodes.py**

```python
"""Graph nodes: planner, coder, refactor, apply, test_runner.

Each make_* factory closes over its dependencies so the graph wiring
stays declarative and tests can inject fakes.
"""
import os
from pathlib import Path

from langchain_core.messages import AIMessage, HumanMessage, SystemMessage

from mac_orchestrator.apply import apply_changes
from mac_orchestrator.parsing import parse_coder_output
from mac_orchestrator.routing import evaluate
from mac_orchestrator.shell import run_test_command
from mac_orchestrator.state import MacAgentState, TDDPhase

PLANNER_SYSTEM = (
    "You are the Planner/Architect of a TDD coding agent. Produce a short, "
    "numbered implementation roadmap. Think first, code second. Simple over "
    "clever. Smallest correct change."
)

CODER_SYSTEM = (
    "You are the Coder of a TDD agent. Reply with a short explanation followed "
    "by exactly one ```json block:\n"
    '{"changes": [{"path": "<relative path>", "content": "<full file content>"}], '
    '"test_command": "<shell command to run the relevant tests>"}\n'
    "Always emit complete file contents, never fragments."
)

REFACTOR_SYSTEM = (
    "You are the Refactor node. Tests are green — improve the implementation "
    "without changing behaviour. Enforce SOLID principles and language-correct "
    "casing (snake_case for Python/Rust/Go, camelCase for JS/TS/Java). Every "
    "function you touch gets a Self-Documenting Prologue comment with fields: "
    "WHY, SCOPE, RESOLVING, THE ISSUE, HOW IT SOLVED, USAGE EXAMPLE (copy-paste "
    "runnable). Reply in the same ```json changes format as the coder."
)


def _usage(reply) -> tuple[int, int]:
    meta = getattr(reply, "usage_metadata", None) or {}
    return meta.get("input_tokens", 0), meta.get("output_tokens", 0)


def build_planner_prompt(task: str, context_map: str,
                         knowledge: list[str], fixes: list[dict[str, str]]) -> str:
    parts = [f"## Task\n{task}"]
    if context_map:
        parts.append(f"## Repository context map\n{context_map}")
    if knowledge:
        parts.append("## Relevant documentation\n" + "\n---\n".join(knowledge))
    if fixes:
        history = "\n".join(f"- ERROR: {f['error']}\n  FIX: {f['fix']}" for f in fixes)
        parts.append(f"## Previous failures in this project (do not repeat)\n{history}")
    return "\n\n".join(parts)


def build_coder_prompt(state: MacAgentState) -> str:
    phase = state["tdd_phase"]
    base = f"## Task\n{state['task']}\n\n## Plan\n{state.get('plan', '')}"
    if phase == TDDPhase.RED:
        return base + (
            "\n\n## Phase: RED\nWrite a high-fidelity failing test for the next "
            "behaviour in the plan. The test MUST fail via an explicit assertion "
            "(not a syntax or import error). Do not write implementation code yet."
        )
    return base + (
        "\n\n## Phase: GREEN\nWrite the MINIMUM implementation to make the failing "
        "test pass. Do not modify the test file."
    )


def make_planner(model, db, project_id: str, session_id: str, embedder=None):
    def planner(state: MacAgentState) -> dict:
        root = Path(state["project_root"])
        context_map = ""
        map_file = root / "CONTEXT_MAP.md"
        if map_file.exists():
            context_map = map_file.read_text()

        knowledge: list[str] = []
        if embedder is not None:
            vector = embedder(state["task"])
            if vector is not None:
                knowledge = db.search_knowledge(vector)

        fixes = db.lookup_fixes(project_id, target_folder=str(root))
        prompt = build_planner_prompt(state["task"], context_map, knowledge, fixes)
        reply = model.invoke([SystemMessage(PLANNER_SYSTEM), HumanMessage(prompt)])

        step = state.get("step_order", 0) + 1
        in_tok, out_tok = _usage(reply)
        db.record_agent_task(session_id, step, "planner",
                             {"prompt": prompt[:4000]}, {"plan": reply.content[:4000]},
                             in_tok, out_tok)
        return {"plan": reply.content, "tdd_phase": TDDPhase.RED,
                "messages": [HumanMessage(prompt), reply], "step_order": step,
                "iterations": 0}
    return planner


def _invoke_for_changes(model, system: str, state: MacAgentState, prompt: str):
    """Call the model; retry once with the parse error appended."""
    messages = [SystemMessage(system), *state.get("messages", []), HumanMessage(prompt)]
    reply = model.invoke(messages)
    try:
        return reply, *parse_coder_output(reply.content)
    except (ValueError, KeyError) as exc:
        retry = messages + [reply, HumanMessage(
            f"Your reply could not be parsed ({exc}). Resend with exactly one "
            "valid ```json block in the required schema.")]
        reply = model.invoke(retry)
        return reply, *parse_coder_output(reply.content)


def make_coder(models: dict, db, project_id: str, session_id: str):
    def coder(state: MacAgentState) -> dict:
        # Architecture-level prompts go to the planner model; iteration to local.
        model = models["coder"]
        prompt = build_coder_prompt(state)
        reply, changes, test_cmd = _invoke_for_changes(model, CODER_SYSTEM, state, prompt)

        step = state.get("step_order", 0) + 1
        in_tok, out_tok = _usage(reply)
        db.record_agent_task(session_id, step, "coder",
                             {"phase": str(state["tdd_phase"])},
                             {"files": [c["path"] for c in changes]}, in_tok, out_tok)
        return {"pending_changes": changes,
                "test_command": test_cmd or state.get("test_command", ""),
                "messages": [reply], "step_order": step}
    return coder


def make_refactor(models: dict, db, project_id: str, session_id: str):
    def refactor(state: MacAgentState) -> dict:
        model = models["planner"]  # refactor benefits from the stronger model
        prompt = (f"## Task\n{state['task']}\n\n## Phase: REFACTOR\n"
                  "Tests are green. Refactor per your system instructions.")
        reply, changes, test_cmd = _invoke_for_changes(model, REFACTOR_SYSTEM, state, prompt)

        step = state.get("step_order", 0) + 1
        in_tok, out_tok = _usage(reply)
        db.record_agent_task(session_id, step, "refactor", {},
                             {"files": [c["path"] for c in changes]}, in_tok, out_tok)
        return {"pending_changes": changes,
                "test_command": test_cmd or state.get("test_command", ""),
                "messages": [reply], "step_order": step}
    return refactor


def make_apply():
    def apply_node(state: MacAgentState) -> dict:
        written = apply_changes(state["project_root"], state.get("pending_changes", []))
        note = AIMessage(content=f"Applied changes to: {', '.join(written) or '(none)'}")
        return {"messages": [note]}
    return apply_node


def make_test_runner(db, project_id: str, session_id: str):
    def test_runner(state: MacAgentState) -> dict:
        result = run_test_command(state["test_command"], cwd=state["project_root"])
        next_phase, verdict, feedback = evaluate(
            TDDPhase(state["tdd_phase"]), result.exit_code, result.output)

        updates: dict = {
            "tdd_phase": next_phase, "verdict": verdict,
            "last_exit_code": result.exit_code, "last_test_output": result.output,
            "iterations": state.get("iterations", 0) + 1,
            "step_order": state.get("step_order", 0) + 1,
        }

        phase = TDDPhase(state["tdd_phase"])
        if phase == TDDPhase.GREEN and result.exit_code != 0:
            # Persist the failure; pull historical fixes into the feedback prompt.
            folder = _primary_folder(state)
            updates["open_error_hash"] = db.save_error(project_id, result.output, folder)
            known = db.lookup_fixes(project_id, error_text=result.output)
            if known:
                feedback += "\n\n## Historical fixes for this exact error\n" + "\n".join(
                    f"- {f['fix']}" for f in known)
        elif phase == TDDPhase.GREEN and result.exit_code == 0 and state.get("open_error_hash"):
            fix_summary = "Fixed by updating: " + ", ".join(
                c["path"] for c in state.get("pending_changes", []))
            db.attach_fix(project_id, state["open_error_hash"], fix_summary)
            updates["open_error_hash"] = ""

        if feedback:
            updates["messages"] = [HumanMessage(feedback)]

        db.record_agent_task(session_id, updates["step_order"], "test_runner",
                             {"command": state["test_command"]},
                             {"exit_code": result.exit_code, "verdict": verdict})
        return updates
    return test_runner


def _primary_folder(state: MacAgentState) -> str:
    changes = state.get("pending_changes", [])
    if changes:
        return os.path.dirname(changes[0]["path"]) or "."
    return "."
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd orchestrator && uv run pytest tests/test_nodes.py -v`
Expected: 6 passed.

- [ ] **Step 5: Run the full suite**

Run: `cd orchestrator && uv run pytest`
Expected: all green (≈30 tests).

- [ ] **Step 6: Commit**

```bash
git add orchestrator/src/mac_orchestrator/nodes.py orchestrator/tests/test_nodes.py
git commit -m "feat(orchestrator): planner, coder, refactor, apply and test-runner nodes"
```

---

### Task 10: Graph wiring with HIL breakpoint

**Files:**
- Create: `orchestrator/src/mac_orchestrator/graph.py`
- Test: `orchestrator/tests/test_graph.py`

- [ ] **Step 1: Write failing tests**

```python
# orchestrator/tests/test_graph.py
from mac_orchestrator.graph import build_graph
from tests.test_nodes import CODER_REPLY, FakeDB, FakeModel


def _compiled():
    models = {"planner": FakeModel("plan"), "coder": FakeModel(CODER_REPLY)}
    return build_graph(models, FakeDB(), project_id="p1", session_id="s1")


def test_graph_has_all_nodes():
    graph = _compiled()
    nodes = set(graph.get_graph().nodes)
    assert {"planner", "coder", "refactor", "apply", "test_runner"} <= nodes


def test_graph_interrupts_before_apply(tmp_path):
    graph = _compiled()
    config = {"configurable": {"thread_id": "t1"}}
    graph.invoke({"project_root": str(tmp_path), "task": "demo",
                  "messages": [], "step_order": 0}, config)
    snapshot = graph.get_state(config)
    assert snapshot.next == ("apply",)
    assert snapshot.values["pending_changes"]  # diff content available for HIL
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd orchestrator && uv run pytest tests/test_graph.py -v`
Expected: FAIL — `ModuleNotFoundError`

- [ ] **Step 3: Implement graph.py**

```python
"""Wire the cyclic TDD StateGraph.

planner -> coder -> [interrupt: HIL] -> apply -> test_runner
test_runner routes: back to coder, on to refactor, or END.
"""
from langgraph.checkpoint.memory import MemorySaver
from langgraph.graph import END, StateGraph

from mac_orchestrator.nodes import (
    make_apply, make_coder, make_planner, make_refactor, make_test_runner,
)
from mac_orchestrator.routing import route_after_test
from mac_orchestrator.state import MacAgentState


def build_graph(models: dict, db, project_id: str, session_id: str, embedder=None):
    g = StateGraph(MacAgentState)
    g.add_node("planner", make_planner(models["planner"], db, project_id, session_id, embedder))
    g.add_node("coder", make_coder(models, db, project_id, session_id))
    g.add_node("refactor", make_refactor(models, db, project_id, session_id))
    g.add_node("apply", make_apply())
    g.add_node("test_runner", make_test_runner(db, project_id, session_id))

    g.set_entry_point("planner")
    g.add_edge("planner", "coder")
    g.add_edge("coder", "apply")
    g.add_edge("refactor", "apply")
    g.add_edge("apply", "test_runner")
    g.add_conditional_edges("test_runner", route_after_test,
                            {"coder": "coder", "refactor": "refactor",
                             "done": END, "halt": END})

    # HIL: pause before any disk write so the operator reviews the diff.
    return g.compile(checkpointer=MemorySaver(), interrupt_before=["apply"])
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd orchestrator && uv run pytest tests/test_graph.py -v`
Expected: 2 passed.

- [ ] **Step 5: Commit**

```bash
git add orchestrator/src/mac_orchestrator/graph.py orchestrator/tests/test_graph.py
git commit -m "feat(orchestrator): TDD state graph with interrupt_before apply"
```

---

### Task 11: CLI entry point with HIL loop

**Files:**
- Create: `orchestrator/src/mac_orchestrator/__main__.py`

No automated test (interactive I/O + live DB); verified manually in Task 12 step 4. Keep all logic that *can* be pure in the modules already tested.

- [ ] **Step 1: Implement __main__.py**

```python
"""mac-orchestrator CLI: drive the TDD graph with human-in-the-loop approval.

Invoked by the Go binary as:
  mac-orchestrator --project <abs path> --task "<task>" --db <postgres dsn>
"""
import argparse
import os
import sys

from mac_orchestrator.apply import render_diff
from mac_orchestrator.config import load_config
from mac_orchestrator.db import Database
from mac_orchestrator.embeddings import embed_or_none
from mac_orchestrator.graph import build_graph
from mac_orchestrator.llm import build_chat_model

DEFAULT_DSN = "postgres://postgres:postgres@localhost:5432/mac_cli?sslmode=disable"


def main() -> int:
    parser = argparse.ArgumentParser(prog="mac-orchestrator")
    parser.add_argument("--project", required=True, help="absolute project root")
    parser.add_argument("--task", required=True, help="coding task prompt")
    parser.add_argument("--db", default=os.environ.get("MAC_DB_URL", DEFAULT_DSN))
    args = parser.parse_args()

    cfg = load_config(args.project)
    db = Database(args.db)
    try:
        return run(db, cfg, args.project, args.task)
    finally:
        db.close()


def run(db: Database, cfg, project_root: str, task: str) -> int:
    project = db.find_project(project_root)
    if project is None:
        print(f"error: no mac project registered at {project_root}", file=sys.stderr)
        return 1
    project_id, name = project
    session_id = db.create_session(project_id, task)
    print(f"Project: {name}\nSession: {session_id}\nTask:    {task}\n")

    models = {"planner": build_chat_model(cfg.planner),
              "coder": build_chat_model(cfg.coder)}
    graph = build_graph(models, db, project_id, session_id, embedder=embed_or_none)
    config = {"configurable": {"thread_id": session_id}}

    state = {"project_root": project_root, "task": task,
             "session_id": session_id, "messages": [], "step_order": 0}
    while True:
        for event in graph.stream(state, config, stream_mode="values"):
            phase = event.get("tdd_phase", "")
            verdict = event.get("verdict", "")
            if phase:
                print(f"  [{phase}] step {event.get('step_order', 0)} {verdict}")
        state = None  # after first run, always resume from checkpoint

        snapshot = graph.get_state(config)
        if not snapshot.next:
            break

        # Interrupted before 'apply' — show diff, ask the human.
        values = snapshot.values
        print("\n" + "=" * 60)
        print(render_diff(project_root, values.get("pending_changes", [])))
        print("=" * 60)
        choice = input("\n[a]pprove & apply  [f]eedback  [q]uit > ").strip().lower()

        if choice.startswith("q"):
            print("Aborted. No changes applied.")
            return 130
        if choice.startswith("f"):
            note = input("feedback > ").strip()
            # Re-enter at coder: planner's successor regenerates with the note.
            graph.update_state(
                config,
                {"messages": [("user", f"Operator feedback on your proposed "
                                       f"changes (regenerate): {note}")],
                 "pending_changes": []},
                as_node="planner",
            )
        # approve: plain resume continues into 'apply'

    final = graph.get_state(config).values
    if final.get("verdict") == "done":
        print(f"\nDone — tests green after {final.get('iterations', 0)} test runs.")
        return 0
    print(f"\nStopped: iteration cap reached "
          f"(last exit code {final.get('last_exit_code')}).", file=sys.stderr)
    return 1


if __name__ == "__main__":
    sys.exit(main())
```

- [ ] **Step 2: Verify it parses and full suite stays green**

Run: `cd orchestrator && uv run python -c "import mac_orchestrator.__main__" && uv run pytest`
Expected: no import errors; all tests pass.

- [ ] **Step 3: Commit**

```bash
git add orchestrator/src/mac_orchestrator/__main__.py
git commit -m "feat(orchestrator): CLI entry with HIL diff approval loop"
```

---

### Task 12: Go handoff — `mac code` execs the orchestrator

**Files:**
- Modify: `cmd/mac/main.go` (replace the Phase-2 stub at lines 71-73; add `resolveDSN` helper)

- [ ] **Step 1: Replace the stub in codeCmd**

In `cmd/mac/main.go`, add `"os/exec"` to imports, then replace the body of `codeCmd`'s `RunE` from the `// Phase 2` comment down:

```go
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			database, err := connectDB(ctx, cmd)
			if err != nil {
				return err
			}
			defer database.Close()

			cwd, _ := os.Getwd()
			project, err := database.FindProjectByPath(ctx, cwd)
			if err != nil {
				return fmt.Errorf("check project: %w", err)
			}
			if project == nil {
				return fmt.Errorf("no mac project in %s — run 'mac' first to initialise", cwd)
			}

			// Hand off to the Python LangGraph orchestrator. It owns the
			// session lifecycle, TDD loop, and HIL prompts on this terminal.
			bin := os.Getenv("MAC_ORCHESTRATOR")
			if bin == "" {
				bin = "mac-orchestrator"
			}
			orch := exec.CommandContext(ctx, bin,
				"--project", cwd,
				"--task", args[0],
				"--db", resolveDSN(cmd),
			)
			orch.Stdin = os.Stdin
			orch.Stdout = os.Stdout
			orch.Stderr = os.Stderr
			if err := orch.Run(); err != nil {
				return fmt.Errorf("orchestrator: %w\n\nInstall it with: uv tool install --from ./orchestrator mac-orchestrator", err)
			}
			return nil
		},
```

- [ ] **Step 2: Extract resolveDSN and reuse it in connectDB**

```go
func resolveDSN(cmd *cobra.Command) string {
	dsn, _ := cmd.Flags().GetString("db")
	if dsn == "" {
		dsn = os.Getenv("MAC_DB_URL")
	}
	if dsn == "" {
		dsn = defaultDSN
	}
	return dsn
}

func connectDB(ctx context.Context, cmd *cobra.Command) (*db.DB, error) {
	database, err := db.Connect(ctx, resolveDSN(cmd))
	if err != nil {
		return nil, fmt.Errorf(
			"cannot connect to PostgreSQL: %w\n\nSet MAC_DB_URL env var or pass --db flag", err)
	}
	return database, nil
}
```

- [ ] **Step 3: Build**

Run: `go build ./...`
Expected: exit 0.

- [ ] **Step 4: Manual smoke test (requires running PostgreSQL + a registered project)**

```bash
cd orchestrator && uv tool install --from . mac-orchestrator --force && cd ..
go build -o mac ./cmd/mac
cd <some registered project>
ANTHROPIC_API_KEY=... <repo>/mac code "add a hello() function returning 'world'"
```
Expected: planner output, RED test diff shown, `[a]pprove` prompt; approving runs tests; loop reaches GREEN/REFACTOR; session + agent_tasks rows appear in DB.

- [ ] **Step 5: Commit**

```bash
git add cmd/mac/main.go
git commit -m "feat(cli): hand off 'mac code' to the LangGraph orchestrator"
```

---

## Self-Review Notes

- **Spec coverage:** MacAgentState (Task 2); planner reads CONTEXT_MAP.md + pgvector + error feedback (Task 9); coder LLM routing from config.toml (Tasks 3/7/9); test runner via subprocess (Task 5); RED clean-failure gate (Task 6 `evaluate`); GREEN failure persistence + historical-fix piping (Task 9 test_runner); REFACTOR with SOLID/casing/prologue (Task 9 REFACTOR_SYSTEM); `interrupt_before` HIL with diff + text feedback + authorization (Tasks 10/11); token metrics into agent_tasks (Task 9 `_usage`).
- **Known compromise:** embeddings need `OPENAI_API_KEY` because the schema fixed VECTOR(1536); lookup degrades gracefully to skipped when unset. The KB *sync* job (daily doc ingestion) is out of Step 2 scope — table read path only.
- **Type consistency check:** `lookup_fixes` signature used identically in nodes/planner and test_runner; `FileChange` keys (`path`, `content`) consistent across parsing/apply/nodes; `verdict` values {"coder","refactor","done","halt"} match routing map in graph.py.
