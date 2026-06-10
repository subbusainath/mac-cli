"""Read the project's .mac/config.toml written by the Go CLI."""
import tomllib
from dataclasses import dataclass
from pathlib import Path


@dataclass(frozen=True)
class AgentLLM:
    provider: str
    model: str
    api_base: str | None = None


@dataclass(frozen=True)
class MacConfig:
    project_name: str
    backend: str
    frontend: str
    planner: AgentLLM
    coder: AgentLLM


def load_config(project_root: str | Path) -> MacConfig:
    path = Path(project_root) / ".mac" / "config.toml"
    raw = tomllib.loads(path.read_text())
    project = raw.get("project", {})
    agents = raw.get("agents", {})

    def agent(name: str, provider: str, model: str) -> AgentLLM:
        section = agents.get(name, {})
        return AgentLLM(
            provider=section.get("provider", provider),
            model=section.get("model", model),
            api_base=section.get("api_base"),
        )

    return MacConfig(
        project_name=project.get("name", ""),
        backend=project.get("backend", ""),
        frontend=project.get("frontend", ""),
        planner=agent("planner", "anthropic", "claude-sonnet-4-6"),
        coder=agent("coder", "local", "qwen2.5-coder:14b"),
    )
