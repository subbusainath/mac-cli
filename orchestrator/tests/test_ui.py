import io
import json

from mac_orchestrator.ui import JsonUI, PlainUI


def test_json_emit_one_line(capsys):
    ui = JsonUI()
    ui.emit(event="phase", phase="RED", iterations=1)
    out = capsys.readouterr().out
    lines = out.strip().splitlines()
    assert len(lines) == 1
    assert json.loads(lines[0]) == {"event": "phase", "phase": "RED", "iterations": 1}


def test_json_read_command(monkeypatch):
    monkeypatch.setattr("sys.stdin", io.StringIO('{"cmd": "feedback", "text": "use a dict"}\n'))
    assert JsonUI().read_command() == {"cmd": "feedback", "text": "use a dict"}


def test_json_read_eof_quits(monkeypatch):
    monkeypatch.setattr("sys.stdin", io.StringIO(""))
    assert JsonUI().read_command() == {"cmd": "quit"}


def test_json_read_garbage_quits(monkeypatch):
    monkeypatch.setattr("sys.stdin", io.StringIO("not json\n"))
    assert JsonUI().read_command() == {"cmd": "quit"}


def test_plain_approve(monkeypatch, capsys):
    monkeypatch.setattr("builtins.input", lambda _: "a")
    ui = PlainUI()
    ui.emit(event="await_approval", changes=[{"path": "x.py", "old": "", "new": "pass\n"}])
    assert ui.read_command() == {"cmd": "approve"}


def test_plain_feedback(monkeypatch):
    answers = iter(["f", "rename it"])
    monkeypatch.setattr("builtins.input", lambda _: next(answers))
    assert PlainUI().read_command() == {"cmd": "feedback", "text": "rename it"}
