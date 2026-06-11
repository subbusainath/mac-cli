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
