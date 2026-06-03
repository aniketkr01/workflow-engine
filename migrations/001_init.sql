-- +migrate Up

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS users (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email       TEXT        NOT NULL UNIQUE,
    password_hash TEXT      NOT NULL,
    role        TEXT        NOT NULL DEFAULT 'operator',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS workflows (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    version     INTEGER     NOT NULL DEFAULT 1,
    owner_id    UUID        NOT NULL REFERENCES users(id),
    status      TEXT        NOT NULL DEFAULT 'draft',
    tasks       JSONB       NOT NULL DEFAULT '[]',
    schedule    JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_workflows_owner_id ON workflows(owner_id);
CREATE INDEX IF NOT EXISTS idx_workflows_status ON workflows(status);

CREATE TABLE IF NOT EXISTS workflow_executions (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id UUID        NOT NULL REFERENCES workflows(id),
    version     INTEGER     NOT NULL,
    status      TEXT        NOT NULL DEFAULT 'pending',
    started_at  TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    outputs     JSONB       DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_executions_workflow_id ON workflow_executions(workflow_id);
CREATE INDEX IF NOT EXISTS idx_executions_status ON workflow_executions(status);

CREATE TABLE IF NOT EXISTS task_executions (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id    UUID        NOT NULL REFERENCES workflow_executions(id),
    workflow_id     UUID        NOT NULL REFERENCES workflows(id),
    task_def_id     TEXT        NOT NULL,
    task_name       TEXT        NOT NULL,
    status          TEXT        NOT NULL DEFAULT 'pending',
    attempt         INTEGER     NOT NULL DEFAULT 0,
    max_attempts    INTEGER     NOT NULL DEFAULT 3,
    input           JSONB       DEFAULT '{}',
    output          JSONB       DEFAULT '{}',
    error           TEXT        DEFAULT '',
    started_at      TIMESTAMPTZ,
    finished_at     TIMESTAMPTZ,
    idempotency_key TEXT        NOT NULL UNIQUE,
    worker_id       TEXT        DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_task_executions_execution_id ON task_executions(execution_id);
CREATE INDEX IF NOT EXISTS idx_task_executions_status ON task_executions(status);
CREATE INDEX IF NOT EXISTS idx_task_executions_idempotency_key ON task_executions(idempotency_key);

CREATE TABLE IF NOT EXISTS mcp_servers (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT        NOT NULL UNIQUE,
    transport   TEXT        NOT NULL DEFAULT 'http',
    endpoint    TEXT        NOT NULL,
    enabled     BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +migrate Down

DROP TABLE IF EXISTS task_executions;
DROP TABLE IF EXISTS workflow_executions;
DROP TABLE IF EXISTS workflows;
DROP TABLE IF EXISTS mcp_servers;
DROP TABLE IF EXISTS users;
