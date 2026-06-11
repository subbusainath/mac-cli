from mac_orchestrator.routing import MAX_ITERATIONS, evaluate, route_after_test
from mac_orchestrator.state import TDDPhase


def test_red_passing_test_loops_back():
    phase, verdict, feedback = evaluate(TDDPhase.RED, exit_code=0, output="2 passed")
    assert phase == TDDPhase.RED
    assert verdict == "coder"
    assert "fail" in feedback.lower()


def test_red_dirty_failure_loops_back():
    phase, verdict, _ = evaluate(TDDPhase.RED, 1, "SyntaxError: invalid syntax")
    assert (phase, verdict) == (TDDPhase.RED, "coder")


def test_red_clean_failure_advances_to_green():
    phase, verdict, _ = evaluate(TDDPhase.RED, 1, "AssertionError: assert 1 == 2")
    assert (phase, verdict) == (TDDPhase.GREEN, "coder")


def test_green_failure_stays_green():
    phase, verdict, _ = evaluate(TDDPhase.GREEN, 1, "AssertionError")
    assert (phase, verdict) == (TDDPhase.GREEN, "coder")


def test_green_pass_advances_to_refactor():
    phase, verdict, _ = evaluate(TDDPhase.GREEN, 0, "3 passed")
    assert (phase, verdict) == (TDDPhase.REFACTOR, "refactor")


def test_refactor_pass_is_done():
    phase, verdict, _ = evaluate(TDDPhase.REFACTOR, 0, "3 passed")
    assert verdict == "done"


def test_refactor_breakage_goes_back_to_green():
    phase, verdict, _ = evaluate(TDDPhase.REFACTOR, 1, "AssertionError")
    assert (phase, verdict) == (TDDPhase.GREEN, "coder")


def test_route_reads_verdict():
    assert route_after_test({"verdict": "coder", "iterations": 1}) == "coder"
    assert route_after_test({"verdict": "done", "iterations": 1}) == "done"


def test_route_halts_at_iteration_cap():
    assert route_after_test({"verdict": "coder", "iterations": MAX_ITERATIONS}) == "halt"
