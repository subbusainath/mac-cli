import pytest

from mac_orchestrator.credentials import lookup_key, require_key


def test_env_wins(monkeypatch, tmp_path):
    monkeypatch.setenv("MAC_CONFIG_DIR", str(tmp_path))
    (tmp_path / "credentials.toml").write_text('openai = "from-file"\n')
    monkeypatch.setenv("OPENAI_API_KEY", "from-env")
    assert lookup_key("openai") == "from-env"


def test_file_fallback(monkeypatch, tmp_path):
    monkeypatch.setenv("MAC_CONFIG_DIR", str(tmp_path))
    monkeypatch.delenv("OPENAI_API_KEY", raising=False)
    (tmp_path / "credentials.toml").write_text('openai = "from-file"\n')
    assert lookup_key("openai") == "from-file"


def test_missing_returns_none(monkeypatch, tmp_path):
    monkeypatch.setenv("MAC_CONFIG_DIR", str(tmp_path))
    monkeypatch.delenv("DEEPSEEK_API_KEY", raising=False)
    assert lookup_key("deepseek") is None


def test_empty_env_does_not_count(monkeypatch, tmp_path):
    monkeypatch.setenv("MAC_CONFIG_DIR", str(tmp_path))
    monkeypatch.setenv("OPENROUTER_API_KEY", "")
    assert lookup_key("openrouter") is None


def test_require_key_raises_with_guidance(monkeypatch, tmp_path):
    monkeypatch.setenv("MAC_CONFIG_DIR", str(tmp_path))
    monkeypatch.delenv("ANTHROPIC_API_KEY", raising=False)
    with pytest.raises(RuntimeError, match="ANTHROPIC_API_KEY"):
        require_key("anthropic")
