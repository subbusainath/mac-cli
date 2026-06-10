import pytest
from langchain_anthropic import ChatAnthropic
from langchain_openai import ChatOpenAI

from mac_orchestrator.config import AgentLLM
from mac_orchestrator.llm import build_chat_model


def test_anthropic_provider(monkeypatch):
    monkeypatch.setenv("ANTHROPIC_API_KEY", "test-key")
    model = build_chat_model(AgentLLM("anthropic", "claude-sonnet-4-6"))
    assert isinstance(model, ChatAnthropic)
    assert model.model == "claude-sonnet-4-6"


def test_local_provider_points_at_ollama():
    model = build_chat_model(
        AgentLLM("local", "qwen2.5-coder:14b", api_base="http://localhost:11434")
    )
    assert isinstance(model, ChatOpenAI)
    assert model.model_name == "qwen2.5-coder:14b"
    assert "11434" in str(model.openai_api_base)


def test_local_provider_default_base():
    model = build_chat_model(AgentLLM("local", "qwen2.5-coder:14b"))
    assert "localhost:11434" in str(model.openai_api_base)


def test_unknown_provider_raises():
    with pytest.raises(ValueError, match="unknown provider"):
        build_chat_model(AgentLLM("watsonx", "x"))
