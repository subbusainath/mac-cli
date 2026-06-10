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
