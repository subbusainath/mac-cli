"""Map .mac/config.toml agent entries to chat models.

'anthropic' -> Claude via ANTHROPIC_API_KEY.
'local'     -> any OpenAI-compatible server (Ollama) at api_base.
"""
from langchain_anthropic import ChatAnthropic
from langchain_openai import ChatOpenAI

from mac_orchestrator.config import AgentLLM

_DEFAULT_OLLAMA = "http://localhost:11434"


def build_chat_model(cfg: AgentLLM):
    if cfg.provider == "anthropic":
        return ChatAnthropic(model=cfg.model, max_tokens=8192)
    if cfg.provider == "local":
        base = (cfg.api_base or _DEFAULT_OLLAMA).rstrip("/")
        return ChatOpenAI(model=cfg.model, base_url=base + "/v1", api_key="ollama")
    raise ValueError(f"unknown provider: {cfg.provider}")
