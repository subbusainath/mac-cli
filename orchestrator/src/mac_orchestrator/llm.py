"""Map .mac/config.toml agent entries to chat models.

'anthropic'  -> Claude via resolved key
'openai'     -> OpenAI via resolved key
'deepseek'   -> DeepSeek's OpenAI-compatible API
'openrouter' -> OpenRouter's OpenAI-compatible API
'local'      -> any OpenAI-compatible server (Ollama) at api_base
"""
from langchain_anthropic import ChatAnthropic
from langchain_openai import ChatOpenAI

from mac_orchestrator.config import AgentLLM
from mac_orchestrator.credentials import require_key

_DEFAULT_OLLAMA = "http://localhost:11434"

_OPENAI_COMPAT_BASES = {
    "deepseek": "https://api.deepseek.com/v1",
    "openrouter": "https://openrouter.ai/api/v1",
}


def build_chat_model(cfg: AgentLLM):
    if cfg.provider == "anthropic":
        return ChatAnthropic(
            model=cfg.model,
            api_key=require_key("anthropic"),
            max_tokens=8192,
        )
    if cfg.provider == "openai":
        return ChatOpenAI(
            model=cfg.model,
            api_key=require_key("openai"),
        )
    if cfg.provider in _OPENAI_COMPAT_BASES:
        return ChatOpenAI(
            model=cfg.model,
            api_key=require_key(cfg.provider),
            base_url=_OPENAI_COMPAT_BASES[cfg.provider],
        )
    if cfg.provider == "local":
        base = (cfg.api_base or _DEFAULT_OLLAMA).rstrip("/")
        return ChatOpenAI(model=cfg.model, base_url=base + "/v1", api_key="ollama")
    raise ValueError(f"unknown provider: {cfg.provider}")
