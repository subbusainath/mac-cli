"""PostgreSQL persistence: sessions, agent task log, error feedback, knowledge base."""
import hashlib
import json
import re
from typing import Any

import psycopg

# Volatile fragments that would break error-signature matching across runs.
_LINE_NO = re.compile(r"line \d+")
_HEX_ADDR = re.compile(r"0x[0-9a-fA-F]+")
_FILE_PATH = re.compile(r'File "[^"]+"')


def normalize_error(text: str) -> str:
    text = _FILE_PATH.sub('File "<path>"', text)
    text = _LINE_NO.sub("line <n>", text)
    text = _HEX_ADDR.sub("<addr>", text)
    return text.strip()


def error_hash(text: str) -> str:
    return hashlib.sha256(normalize_error(text).encode()).hexdigest()


def to_vector_literal(embedding: list[float]) -> str:
    return "[" + ",".join(repr(x) for x in embedding) + "]"


class Database:
    def __init__(self, dsn: str):
        self.conn = psycopg.connect(dsn, autocommit=True)

    def close(self) -> None:
        self.conn.close()

    def find_project(self, path: str) -> tuple[str, str] | None:
        row = self.conn.execute(
            "SELECT id, name FROM projects WHERE path = %s", (path,)
        ).fetchone()
        return (str(row[0]), row[1]) if row else None

    def create_session(self, project_id: str, user_prompt: str) -> str:
        row = self.conn.execute(
            "INSERT INTO sessions (project_id, user_prompt) VALUES (%s, %s) RETURNING id",
            (project_id, user_prompt),
        ).fetchone()
        return str(row[0])

    def record_agent_task(
        self,
        session_id: str,
        step_order: int,
        node_name: str,
        input_payload: dict[str, Any],
        output_payload: dict[str, Any],
        prompt_tokens: int = 0,
        completion_tokens: int = 0,
    ) -> None:
        self.conn.execute(
            """INSERT INTO agent_tasks
               (session_id, step_order, node_name, input_payload, output_payload,
                prompt_tokens, completion_tokens)
               VALUES (%s, %s, %s, %s, %s, %s, %s)""",
            (session_id, step_order, node_name, json.dumps(input_payload),
             json.dumps(output_payload), prompt_tokens, completion_tokens),
        )

    def save_error(self, project_id: str, error_text: str, target_folder: str) -> str:
        """Record a GREEN-phase failure; returns its hash. Fix attached later."""
        h = error_hash(error_text)
        self.conn.execute(
            """INSERT INTO terminal_errors_feedback
               (project_id, error_hash, error_signature, successful_fix, target_folder)
               VALUES (%s, %s, %s, '', %s)
               ON CONFLICT (project_id, error_hash) DO NOTHING""",
            (project_id, h, normalize_error(error_text)[:4000], target_folder),
        )
        return h

    def attach_fix(self, project_id: str, h: str, fix: str) -> None:
        self.conn.execute(
            """UPDATE terminal_errors_feedback SET successful_fix = %s
               WHERE project_id = %s AND error_hash = %s""",
            (fix[:8000], project_id, h),
        )

    def lookup_fixes(self, project_id: str, error_text: str | None = None,
                     target_folder: str | None = None, limit: int = 5) -> list[dict[str, str]]:
        """Historical fixes: exact hash match first, then same-folder matches."""
        clauses, params = ["project_id = %s", "successful_fix <> ''"], [project_id]
        if error_text is not None:
            clauses.append("error_hash = %s")
            params.append(error_hash(error_text))
        elif target_folder is not None:
            clauses.append("target_folder = %s")
            params.append(target_folder)
        rows = self.conn.execute(
            f"""SELECT error_signature, successful_fix FROM terminal_errors_feedback
                WHERE {' AND '.join(clauses)} ORDER BY created_at DESC LIMIT %s""",
            (*params, limit),
        ).fetchall()
        return [{"error": r[0], "fix": r[1]} for r in rows]

    def search_knowledge(self, embedding: list[float], limit: int = 5) -> list[str]:
        rows = self.conn.execute(
            """SELECT content FROM technical_knowledge_base
               WHERE embedding IS NOT NULL
               ORDER BY embedding <=> %s::vector LIMIT %s""",
            (to_vector_literal(embedding), limit),
        ).fetchall()
        return [r[0] for r in rows]
