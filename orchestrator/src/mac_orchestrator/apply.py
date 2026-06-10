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
