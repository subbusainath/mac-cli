"""Resolve provider API keys: env var first, then ~/.config/mac/credentials.toml.

Mirrors the Go internal/credentials package, including the MAC_CONFIG_DIR
test override.
"""
import os
import tomllib
from pathlib import Path

ENV_VARS = {
    "openai": "OPENAI_API_KEY",
    "anthropic": "ANTHROPIC_API_KEY",
    "deepseek": "DEEPSEEK_API_KEY",
    "openrouter": "OPENROUTER_API_KEY",
}


def _credentials_path() -> Path:
    base = os.environ.get("MAC_CONFIG_DIR")
    root = Path(base) if base else Path.home() / ".config" / "mac"
    return root / "credentials.toml"


def lookup_key(provider: str) -> str | None:
    env = ENV_VARS.get(provider)
    if env:
        val = os.environ.get(env, "").strip()
        if val:
            return val
    path = _credentials_path()
    if path.exists():
        data = tomllib.loads(path.read_text())
        val = str(data.get(provider, "")).strip()
        if val:
            return val
    return None


def require_key(provider: str) -> str:
    key = lookup_key(provider)
    if key is None:
        env = ENV_VARS.get(provider, "<unknown>")
        raise RuntimeError(
            f"no API key for provider '{provider}'. Set {env} or store the key "
            f"in {_credentials_path()} (re-running the 'mac' wizard can do this)."
        )
    return key
