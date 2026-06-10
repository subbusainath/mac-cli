"""Optional task embeddings for pgvector knowledge lookup.

The knowledge base uses VECTOR(1536) = OpenAI text-embedding-3-small.
Without OPENAI_API_KEY the lookup is skipped gracefully.
"""
import os


def embed_or_none(text: str) -> list[float] | None:
    if not os.environ.get("OPENAI_API_KEY"):
        return None
    from langchain_openai import OpenAIEmbeddings

    embedder = OpenAIEmbeddings(model="text-embedding-3-small")
    return embedder.embed_query(text)
