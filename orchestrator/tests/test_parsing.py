import pytest

from mac_orchestrator.parsing import parse_coder_output

REPLY = '''Here is the change.

```json
{
  "changes": [
    {"path": "tests/test_auth.py", "content": "def test_x():\\n    assert add(1, 1) == 2\\n"}
  ],
  "test_command": "uv run pytest tests/test_auth.py -v"
}
```
'''


def test_parses_changes_and_command():
    changes, cmd = parse_coder_output(REPLY)
    assert changes == [
        {"path": "tests/test_auth.py", "content": "def test_x():\n    assert add(1, 1) == 2\n"}
    ]
    assert cmd == "uv run pytest tests/test_auth.py -v"


def test_missing_json_block_raises():
    with pytest.raises(ValueError, match="json"):
        parse_coder_output("no block here")


def test_missing_test_command_defaults_empty():
    reply = '```json\n{"changes": []}\n```'
    changes, cmd = parse_coder_output(reply)
    assert changes == []
    assert cmd == ""
