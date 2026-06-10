import pytest

from mac_orchestrator.apply import apply_changes, render_diff


def test_apply_writes_new_file(tmp_path):
    changes = [{"path": "src/util.py", "content": "x = 1\n"}]
    written = apply_changes(str(tmp_path), changes)
    assert written == ["src/util.py"]
    assert (tmp_path / "src" / "util.py").read_text() == "x = 1\n"


def test_apply_overwrites_existing(tmp_path):
    (tmp_path / "a.py").write_text("old\n")
    apply_changes(str(tmp_path), [{"path": "a.py", "content": "new\n"}])
    assert (tmp_path / "a.py").read_text() == "new\n"


def test_apply_rejects_path_escape(tmp_path):
    with pytest.raises(ValueError, match="outside project root"):
        apply_changes(str(tmp_path), [{"path": "../evil.py", "content": ""}])


def test_diff_shows_old_and_new(tmp_path):
    (tmp_path / "a.py").write_text("old line\n")
    diff = render_diff(str(tmp_path), [{"path": "a.py", "content": "new line\n"}])
    assert "-old line" in diff
    assert "+new line" in diff


def test_diff_new_file_all_additions(tmp_path):
    diff = render_diff(str(tmp_path), [{"path": "b.py", "content": "added\n"}])
    assert "+added" in diff
    assert "(new file)" in diff
