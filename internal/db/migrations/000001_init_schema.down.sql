DROP INDEX IF EXISTS idx_knowledge_embedding;
DROP INDEX IF EXISTS idx_errors_project_hash;
DROP INDEX IF EXISTS idx_agent_tasks_session_step;
DROP INDEX IF EXISTS idx_sessions_project_id;
DROP INDEX IF EXISTS idx_projects_path;

DROP TABLE IF EXISTS technical_knowledge_base;
DROP TABLE IF EXISTS terminal_errors_feedback;
DROP TABLE IF EXISTS agent_tasks;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS projects;

DROP EXTENSION IF EXISTS vector;
