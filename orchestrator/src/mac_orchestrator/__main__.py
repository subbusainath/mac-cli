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
