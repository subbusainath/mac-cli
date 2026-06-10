from mac_orchestrator.shell import classify_failure, run_test_command


def test_run_captures_exit_and_output(tmp_path):
    result = run_test_command("echo hello && exit 3", cwd=str(tmp_path))
    assert result.exit_code == 3
    assert "hello" in result.output


def test_run_success(tmp_path):
    result = run_test_command("true", cwd=str(tmp_path))
    assert result.exit_code == 0


def test_classify_clean_assertion_failure():
    out = "FAILED tests/test_x.py::test_x - AssertionError: assert 1 == 2"
    assert classify_failure(out) == "clean"


def test_classify_dirty_syntax_error():
    out = 'E   SyntaxError: invalid syntax\nERROR collecting tests/test_x.py'
    assert classify_failure(out) == "dirty"


def test_classify_dirty_missing_command():
    assert classify_failure("zsh: command not found: pytset") == "dirty"


def test_classify_unknown_failure_treated_clean():
    assert classify_failure("1 test failed: expected 200 got 500") == "clean"
