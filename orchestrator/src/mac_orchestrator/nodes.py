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
