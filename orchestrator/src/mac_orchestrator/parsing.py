"""Extract structured file changes from coder LLM replies."""
import json
import re

from mac_orchestrator.state import FileChange

_JSON_BLOCK = re.compile(r"```json\s*(\{.*?\})\s*```", re.DOTALL)


def parse_coder_output(text: str) -> tuple[list[FileChange], str]:
    match = _JSON_BLOCK.search(text)
    if match is None:
        raise ValueError("coder reply has no ```json block with changes")
    try:
        data = json.loads(match.group(1))
    except json.JSONDecodeError as exc:
        raise ValueError(f"coder reply ```json block is not valid JSON: {exc}") from exc
    changes: list[FileChange] = [
        {"path": c["path"], "content": c["content"]} for c in data.get("changes", [])
    ]
    return changes, data.get("test_command", "")
