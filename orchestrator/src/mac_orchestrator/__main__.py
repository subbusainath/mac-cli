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
