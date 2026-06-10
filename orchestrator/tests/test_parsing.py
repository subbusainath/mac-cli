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


def test_nested_multikey_json_with_trailing_prose():
    reply = (
        "Explanation.\n```json\n"
        '{"changes": [{"path": "a.py", "content": "x={1:2}"},'
        ' {"path": "b/c.py", "content": "y"}], "test_command": "pytest -v"}'
        "\n```\nMore prose after."
    )
    changes, cmd = parse_coder_output(reply)
    assert [c["path"] for c in changes] == ["a.py", "b/c.py"]
    assert cmd == "pytest -v"


def test_invalid_json_in_block_raises_value_error():
    with pytest.raises(ValueError, match="not valid JSON"):
        parse_coder_output('```json\n{"changes": [,]}\n```')


def test_missing_test_command_defaults_empty():
    reply = '```json\n{"changes": []}\n```'
    changes, cmd = parse_coder_output(reply)
    assert changes == []
    assert cmd == ""
