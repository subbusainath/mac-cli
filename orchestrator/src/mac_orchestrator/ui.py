"""UI channels for the orchestrator CLI.

JsonUI speaks the line-delimited JSON protocol consumed by the Go TUI.
PlainUI keeps the legacy human-readable terminal behaviour.
"""
import json
import sys


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
            print("\n" + "=" * 60)
            for c in event.get("changes", []):
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
