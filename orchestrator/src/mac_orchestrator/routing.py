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
    verdict = state.get("verdict", "halt")
    if state.get("iterations", 0) >= MAX_ITERATIONS and verdict != "done":
        return "halt"
    return verdict
