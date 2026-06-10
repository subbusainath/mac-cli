CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS projects (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT        NOT NULL,
    path       TEXT        NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sessions (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id     UUID        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_prompt    TEXT        NOT NULL,
    global_context JSONB       NOT NULL DEFAULT '{}',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS agent_tasks (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id        UUID        NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    step_order        INT         NOT NULL,
    node_name         TEXT        NOT NULL,
    input_payload     JSONB       NOT NULL DEFAULT '{}',
    output_payload    JSONB       NOT NULL DEFAULT '{}',
    prompt_tokens     INT         NOT NULL DEFAULT 0,
    completion_tokens INT         NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS terminal_errors_feedback (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id       UUID        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    error_hash       TEXT        NOT NULL,
    error_signature  TEXT        NOT NULL,
    successful_fix   TEXT        NOT NULL,
    target_folder    TEXT        NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (project_id, error_hash)
);

CREATE TABLE IF NOT EXISTS technical_knowledge_base (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    content       TEXT        NOT NULL,
    embedding     VECTOR(1536),
    source_url    TEXT,
    last_synced_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_projects_path
    ON projects (path);

CREATE INDEX IF NOT EXISTS idx_sessions_project_id
    ON sessions (project_id);

CREATE INDEX IF NOT EXISTS idx_agent_tasks_session_step
    ON agent_tasks (session_id, step_order);

CREATE INDEX IF NOT EXISTS idx_errors_project_hash
    ON terminal_errors_feedback (project_id, error_hash);

CREATE INDEX IF NOT EXISTS idx_knowledge_embedding
    ON technical_knowledge_base USING ivfflat (embedding vector_cosine_ops)
    WITH (lists = 100);
