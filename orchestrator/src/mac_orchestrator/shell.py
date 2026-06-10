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
