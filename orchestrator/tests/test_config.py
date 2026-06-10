from pathlib import Path

from mac_orchestrator.config import load_config

SAMPLE = """
[project]
name = "demo"
backend = "fastapi"
frontend = "nextjs"
cloud = "aws"
iac = "terraform"

[agents.planner]
provider = "anthropic"
model = "claude-sonnet-4-6"

[agents.coder]
provider = "local"
model = "qwen2.5-coder:14b"
api_base = "http://localhost:11434"
"""


def write_cfg(root: Path, body: str) -> None:
    (root / ".mac").mkdir()
    (root / ".mac" / "config.toml").write_text(body)


def test_loads_full_config(tmp_path):
    write_cfg(tmp_path, SAMPLE)
    cfg = load_config(tmp_path)
    assert cfg.project_name == "demo"
    assert cfg.backend == "fastapi"
    assert cfg.planner.provider == "anthropic"
    assert cfg.coder.model == "qwen2.5-coder:14b"
    assert cfg.coder.api_base == "http://localhost:11434"


def test_missing_agents_fall_back_to_defaults(tmp_path):
    write_cfg(tmp_path, '[project]\nname = "x"\n')
    cfg = load_config(tmp_path)
    assert cfg.planner.provider == "anthropic"
    assert cfg.coder.provider == "local"
    assert cfg.coder.api_base is None
