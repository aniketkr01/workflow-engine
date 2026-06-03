# Workflow Engine

<div align="center">
<img width="724" height="543" alt="workflow-engine-logo" src="https://github.com/user-attachments/assets/efadd963-3fcd-484a-815d-758fb1f3ce16" />
</div>

A production-grade distributed workflow orchestration engine built in Go. Define workflows as DAGs (Directed Acyclic Graphs), execute tasks across a scalable worker pool, integrate with MCP (Model Context Protocol) tools, and observe everything in real-time.

## Table of Contents

- [Features](#features)
- [Architecture](#architecture)
- [Tech Stack](#tech-stack)
- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Local Development](#local-development)
- [Configuration](#configuration)
- [API Reference](#api-reference)
- [Workflow Definition](#workflow-definition)
- [MCP Tool Integration](#mcp-tool-integration)
- [Observability](#observability)
- [Project Structure](#project-structure)
- [Database Schema](#database-schema)

---

## Features

- **DAG-based execution** — Define task dependencies; the engine resolves order automatically using Kahn's topological sort algorithm with cycle detection.
- **Distributed worker pool** — Horizontally scalable workers consume tasks from Redis Streams with configurable concurrency.
- **MCP tool integration** — Dynamically discover and invoke tools from any registered MCP server (HTTP or stdio transport).
- **Fault-tolerant retries** — Per-task retry policies with configurable max attempts; exhausted tasks are moved to a dead-letter queue.
- **Idempotent dispatch** — Tasks carry idempotency keys so duplicate dispatches are safely ignored.
- **REST + gRPC APIs** — HTTP API for user-facing operations; gRPC for internal/programmatic access.
- **JWT authentication** — Stateless auth with role-based access control (admin, operator, viewer).
- **Full observability** — Structured logging (Uber Zap), Prometheus metrics, and OpenTelemetry distributed tracing (Jaeger).
- **Graceful shutdown** — In-flight tasks finish before the process exits.

---

## Architecture

## Architecture

```mermaid
flowchart LR
    Client[Client / User] -->|REST / gRPC| API[API Service]

    API --> Auth[JWT Auth + RBAC]
    API --> Orchestrator[Workflow Orchestrator]

    Orchestrator --> DAG[DAG Validation<br/>Cycle Detection<br/>Topological Sort]
    Orchestrator --> Scheduler[Task Scheduler]
    Scheduler -->|Enqueue ready tasks| Redis[(Redis Streams<br/>Task Queue)]
    Redis --> DLQ[(Dead Letter Queue)]

    Redis -->|Consume tasks| WorkerPool[Distributed Worker Pool]

    WorkerPool --> Worker1[Worker 1]
    WorkerPool --> Worker2[Worker 2]
    WorkerPool --> WorkerN[Worker N]

    Worker1 --> Executor[Task Executor]
    Worker2 --> Executor
    WorkerN --> Executor

    Executor -->|HTTP task| ExternalAPI[External HTTP APIs]
    Executor -->|MCP tool task| MCP[MCP Tool Servers]
    MCP --> MCPHTTP[HTTP Transport]
    MCP --> MCPStdio[Stdio Transport]

    API --> DB[(PostgreSQL)]
    Orchestrator --> DB
    WorkerPool --> DB

    DB --> Users[Users]
    DB --> Workflows[Workflows]
    DB --> Executions[Workflow Executions]
    DB --> Tasks[Task Executions]
    DB --> MCPRegistry[MCP Server Registry]

    API --> Logs[Structured Logs<br/>Uber Zap]
    WorkerPool --> Logs

    API --> Metrics[Prometheus Metrics]
    WorkerPool --> Metrics

    API --> Traces[OpenTelemetry Traces<br/>Jaeger]
    WorkerPool --> Traces

    Metrics --> Grafana[Grafana Dashboards]
```

### Execution Flow

1. **Create workflow** — POST a workflow definition (JSON with task DAG) via REST API.
2. **Trigger** — POST `/workflows/:id/trigger` → Orchestrator creates a `WorkflowExecution`, validates the DAG, and enqueues tasks that have no dependencies.
3. **Worker picks up task** — Worker polls Redis Streams, marks task as `running`, executes it (MCP tool call or HTTP), then marks it `completed` or `failed`.
4. **Orchestrator advances** — On completion, the orchestrator checks which downstream tasks are now unblocked and enqueues them; dependency outputs are automatically forwarded as inputs.
5. **Retry / DLQ** — Failed tasks are re-enqueued up to `maxAttempts`; on exhaustion they go to the dead-letter queue and the workflow is marked `failed`.
6. **Completion** — When all tasks complete, the workflow execution is marked `completed`.

---

## Tech Stack

| Layer | Technology |
|---|---|
| Language | Go 1.26 |
| HTTP framework | [Gin](https://github.com/gin-gonic/gin) |
| RPC | gRPC |
| Database | PostgreSQL 16 (pgx/v5, JSONB for workflow definitions) |
| Message queue | Redis 7 Streams |
| Auth | JWT (golang-jwt/jwt v5), bcrypt |
| Logging | [Uber Zap](https://github.com/uber-go/zap) |
| Metrics | [Prometheus](https://prometheus.io/) + Grafana |
| Tracing | OpenTelemetry → Jaeger |
| Containers | Docker + Docker Compose |

---

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and [Docker Compose](https://docs.docker.com/compose/install/)
- Go 1.26+ (for local development only)

---

## Quick Start

Clone the repository and start everything with Docker Compose:

```bash
git clone https://github.com/aniketkr01/workflow-engine.git
cd workflow-engine
docker compose up -d
```

This starts:

| Service | URL |
|---|---|
| API | http://localhost:8080 |
| gRPC | localhost:9090 |
| Jaeger UI | http://localhost:16686 |
| Prometheus | http://localhost:9091 |
| Grafana | http://localhost:3000 (admin/admin) |
| PostgreSQL | localhost:5432 |
| Redis | localhost:6379 |

Verify the API is healthy:

```bash
curl http://localhost:8080/healthz
# {"status":"ok"}
```

### First Workflow in 60 Seconds

```bash
# 1. Register a user
curl -s -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"password123","role":"admin"}' \
  | jq .

# 2. Save the token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"password123"}' \
  | jq -r .token)

# 3. Create a workflow
WORKFLOW_ID=$(curl -s -X POST http://localhost:8080/api/v1/workflows \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Hello World",
    "description": "A simple two-step workflow",
    "tasks": [
      {
        "id": "step-1",
        "name": "Step 1",
        "type": "http",
        "parameters": {"__type": "http", "url": "https://httpbin.org/get"},
        "dependencies": []
      },
      {
        "id": "step-2",
        "name": "Step 2",
        "type": "http",
        "parameters": {"__type": "http"},
        "dependencies": ["step-1"]
      }
    ]
  }' | jq -r .id)

# 4. Trigger the workflow
curl -s -X POST "http://localhost:8080/api/v1/workflows/$WORKFLOW_ID/trigger" \
  -H "Authorization: Bearer $TOKEN" | jq .

# 5. Check execution status
EXEC_ID="<execution_id_from_above>"
curl -s "http://localhost:8080/api/v1/executions/$EXEC_ID" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

---

## Local Development

### Setup

```bash
# Start infrastructure only (Postgres + Redis + observability stack)
docker compose up -d postgres redis otel-collector jaeger prometheus grafana

# Install Go dependencies
go mod download

# Run the API server
DATABASE_URL="postgres://postgres:postgres@localhost:5432/workflow_engine?sslmode=disable" \
REDIS_ADDR="localhost:6379" \
JWT_SECRET="dev-secret" \
go run ./cmd/api

# In a separate terminal, run a worker
DATABASE_URL="postgres://postgres:postgres@localhost:5432/workflow_engine?sslmode=disable" \
REDIS_ADDR="localhost:6379" \
go run ./cmd/worker
```

### Building

```bash
# Build binaries
go build -o bin/api ./cmd/api
go build -o bin/worker ./cmd/worker

# Build Docker images
docker build -f Dockerfile.api -t workflow-engine-api .
docker build -f Dockerfile.worker -t workflow-engine-worker .
```

---

## Configuration

All configuration is via environment variables.

### API Service

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | `postgres://postgres:postgres@localhost:5432/workflow_engine?sslmode=disable` | PostgreSQL connection string |
| `REDIS_ADDR` | `localhost:6379` | Redis address |
| `REDIS_PASSWORD` | `` | Redis password (empty = no auth) |
| `REDIS_DB` | `0` | Redis database index |
| `JWT_SECRET` | `change-me` | HMAC secret for JWT signing — **change in production** |
| `JWT_TOKEN_DURATION` | `24h` | JWT token lifetime |
| `HTTP_PORT` | `8080` | HTTP listen port |
| `GRPC_PORT` | `9090` | gRPC listen port |
| `SHUTDOWN_TIMEOUT` | `15s` | Graceful shutdown timeout |
| `ENVIRONMENT` | `development` | Deployment environment tag |
| `SERVICE_NAME` | `workflow-engine` | Service name in telemetry |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://localhost:4317` | OpenTelemetry Collector OTLP endpoint |
| `TRACING_ENABLED` | `true` | Enable distributed tracing |
| `TASK_QUEUE_NAME` | `workflow:tasks` | Redis Streams task queue name |
| `TASK_DLQ_NAME` | `workflow:dlq` | Redis Streams dead-letter queue name |

### Worker Service

All database/Redis/telemetry variables above apply, plus:

| Variable | Default | Description |
|---|---|---|
| `WORKER_CONCURRENCY` | `10` | Max concurrent tasks per worker process |
| `WORKER_POLL_INTERVAL` | `500ms` | Redis Streams block timeout |

---

## API Reference

All API routes are prefixed with `/api/v1`. Authenticated routes require `Authorization: Bearer <token>`.

### Auth

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/auth/register` | Public | Register a new user |
| `POST` | `/auth/login` | Public | Login and receive JWT |
| `GET` | `/auth/me` | Required | Get current user info |

**Register / Login request body:**
```json
{
  "email": "user@example.com",
  "password": "password123",
  "role": "operator"   // register only: admin | operator | viewer
}
```

### Workflows

| Method | Path | Roles | Description |
|---|---|---|---|
| `POST` | `/workflows` | Any | Create a workflow |
| `GET` | `/workflows` | Any | List workflows (paginated) |
| `GET` | `/workflows/:id` | Any | Get workflow by ID |
| `PUT` | `/workflows/:id` | Any | Update a workflow |
| `DELETE` | `/workflows/:id` | admin, operator | Delete a workflow |
| `POST` | `/workflows/:id/trigger` | Any | Trigger workflow execution |
| `GET` | `/workflows/:id/executions` | Any | List executions for a workflow |

**Pagination query params:** `?limit=20&offset=0`

### Executions

| Method | Path | Description |
|---|---|---|
| `GET` | `/executions/:id` | Get execution status |
| `POST` | `/executions/:id/cancel` | Cancel a running execution |
| `GET` | `/executions/:id/tasks` | List all task executions |

### MCP Servers

| Method | Path | Roles | Description |
|---|---|---|---|
| `POST` | `/mcp/servers` | admin | Register an MCP server |
| `GET` | `/mcp/servers` | admin | List all MCP servers |
| `DELETE` | `/mcp/servers/:id` | admin | Remove an MCP server |
| `GET` | `/mcp/tools` | Any | List all available tools |

### System

| Method | Path | Description |
|---|---|---|
| `GET` | `/healthz` | Health check |
| `GET` | `/metrics` | Prometheus metrics |

---

## Workflow Definition

Workflows are defined as JSON. The `tasks` array describes the DAG.

```json
{
  "name": "Data Pipeline",
  "description": "Fetch, transform and store data",
  "tasks": [
    {
      "id": "fetch-data",
      "name": "Fetch Data",
      "type": "mcp_tool",
      "parameters": {
        "__type": "mcp_tool",
        "__mcp_server": "my-data-server",
        "__tool_name": "fetch_records",
        "dataset": "sales_2024",
        "limit": 1000
      },
      "dependencies": [],
      "timeout_sec": 60,
      "retry_policy": {
        "max_attempts": 3
      }
    },
    {
      "id": "transform-data",
      "name": "Transform Data",
      "type": "mcp_tool",
      "parameters": {
        "__type": "mcp_tool",
        "__mcp_server": "my-data-server",
        "__tool_name": "transform_records"
      },
      "dependencies": ["fetch-data"],
      "input_mapping": {
        "fetch-data.result": "raw_data"
      },
      "timeout_sec": 120,
      "retry_policy": {
        "max_attempts": 2
      }
    },
    {
      "id": "store-data",
      "name": "Store Data",
      "type": "mcp_tool",
      "parameters": {
        "__type": "mcp_tool",
        "__mcp_server": "my-data-server",
        "__tool_name": "store_records"
      },
      "dependencies": ["transform-data"],
      "timeout_sec": 30
    }
  ]
}
```

### Task Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `id` | string | Yes | Unique task identifier within the workflow |
| `name` | string | Yes | Human-readable task name |
| `type` | string | No | `mcp_tool`, `http`, or `script` (default: `mcp_tool` if `__mcp_server` is set) |
| `parameters` | object | No | Static input parameters merged with dependency outputs |
| `dependencies` | []string | No | List of task IDs that must complete before this task runs |
| `timeout_sec` | int | No | Per-task execution timeout in seconds (default: 300) |
| `retry_policy.max_attempts` | int | No | Max total attempts including the first (default: 3) |
| `input_mapping` | object | No | Map dependency output keys to input keys (`"dep_id.output_key": "input_key"`) |

### Reserved Parameter Keys

| Key | Description |
|---|---|
| `__type` | Task type: `mcp_tool` or `http` |
| `__mcp_server` | MCP server name to invoke |
| `__tool_name` | Tool name on the MCP server |

### Dependency Output Forwarding

When a task completes, its outputs are automatically available to downstream tasks. Use `input_mapping` to rename keys:

```json
"input_mapping": {
  "fetch-data.result": "raw_data",
  "fetch-data.count":  "record_count"
}
```

Without a mapping, all output keys from dependencies are merged into the task's input as-is.

---

## MCP Tool Integration

Register any MCP-compatible server to make its tools available to workflows.

### Register an MCP Server

```bash
curl -X POST http://localhost:8080/api/v1/mcp/servers \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-tools",
    "transport": "http",
    "endpoint": "http://my-mcp-server:8000"
  }'
```

**Transport options:**

| Transport | Description |
|---|---|
| `http` | MCP server accessible via HTTP (JSON-RPC over HTTP) |
| `stdio` | MCP server launched as a subprocess (command in endpoint field) |

### List Available Tools

```bash
curl http://localhost:8080/api/v1/mcp/tools \
  -H "Authorization: Bearer $TOKEN"
```

### Using MCP Tools in Workflows

Reference a registered server and tool in task parameters:

```json
"parameters": {
  "__type": "mcp_tool",
  "__mcp_server": "my-tools",
  "__tool_name": "run_query",
  "query": "SELECT * FROM orders WHERE date > '2024-01-01'"
}
```

The worker resolves the MCP client from the registry, strips the `__`-prefixed keys, and calls the tool with the remaining parameters as arguments.

---

## Observability

### Logs

Structured JSON logs via Uber Zap. In development (`ENVIRONMENT=development`) logs are pretty-printed. Example:

```json
{"level":"info","service":"workflow-engine","time":"2024-01-15T10:00:00Z","port":"8080","message":"HTTP server starting"}
```

### Metrics

Prometheus metrics are exposed at `GET /metrics`. Key metrics:

| Metric | Type | Description |
|---|---|---|
| `workflow_engine_workflows_started_total` | Counter | Workflows triggered |
| `workflow_engine_workflows_completed_total` | Counter | Workflows completed successfully |
| `workflow_engine_workflows_failed_total` | Counter | Workflows that failed |
| `workflow_engine_tasks_dispatched_total` | Counter | Tasks sent to queue |
| `workflow_engine_tasks_completed_total` | Counter | Tasks completed |
| `workflow_engine_tasks_failed_total` | Counter | Tasks failed |
| `workflow_engine_task_retries_total` | Counter | Task retry attempts |
| `workflow_engine_tasks_dead_total` | Counter | Tasks moved to DLQ |
| `workflow_engine_tasks_running` | Gauge | Currently executing tasks |
| `workflow_engine_queue_depth` | Gauge | Pending messages in task queue |
| `workflow_engine_http_request_duration_seconds` | Histogram | HTTP request latency by method/path/status |

**Grafana** is available at http://localhost:3000. Add Prometheus (`http://prometheus:9090`) as a data source to build dashboards.

### Tracing

Distributed traces are sent via OpenTelemetry to Jaeger. View traces at http://localhost:16686. Each HTTP request and task execution is traced.

---

## Project Structure

```
.
├── cmd/
│   ├── api/            # API server entry point
│   └── worker/         # Worker pool entry point
├── internal/
│   ├── api/
│   │   ├── handlers/   # HTTP handlers (auth, workflow, mcp)
│   │   ├── middleware/  # Gin middleware (logging, metrics, recovery, correlation ID)
│   │   └── router.go   # Route definitions
│   ├── auth/           # JWT generation/verification, auth middleware
│   ├── config/         # Environment-based configuration loader
│   ├── domain/         # Domain models (Workflow, TaskExecution, User)
│   ├── engine/
│   │   ├── dag.go          # DAG construction and cycle detection (Kahn's algorithm)
│   │   ├── orchestrator.go # Workflow execution lifecycle, task dispatch and retry
│   │   └── scheduler.go    # Periodic check for stuck/pending executions
│   ├── grpcserver/     # gRPC server (TriggerWorkflow, GetExecution, GetTask RPCs)
│   ├── mcp/
│   │   ├── client.go    # MCP protocol client (HTTP transport)
│   │   ├── protocol.go  # JSON-RPC request/response types
│   │   └── registry.go  # MCP server registry, tool resolution
│   ├── queue/          # Redis Streams wrapper (enqueue, consume, ack, DLQ)
│   ├── repository/
│   │   ├── interfaces.go   # Repository interfaces
│   │   └── postgres/       # PostgreSQL implementations
│   ├── telemetry/      # Uber Zap logger, Prometheus metrics, OpenTelemetry tracing
│   └── worker/         # Worker pool (semaphore-based concurrency, task execution)
├── migrations/
│   └── 001_init.sql    # Database schema (auto-applied by Docker Compose)
├── proto/
│   └── workflow.proto  # gRPC service and message definitions
├── docker-compose.yml
├── Dockerfile.api
├── Dockerfile.worker
├── otel-collector-config.yaml
└── prometheus.yml
```

---

## Database Schema

Five tables in PostgreSQL:

```
users
  id, email, password_hash, role, created_at, updated_at, deleted_at

workflows
  id, name, description, version, owner_id → users, status, tasks (JSONB),
  schedule (JSONB), created_at, updated_at

workflow_executions
  id, workflow_id → workflows, version, status, started_at, finished_at,
  outputs (JSONB), created_at, updated_at

task_executions
  id, execution_id → workflow_executions, workflow_id → workflows,
  task_def_id, task_name, status, attempt, max_attempts,
  input (JSONB), output (JSONB), error, started_at, finished_at,
  idempotency_key (UNIQUE), worker_id, created_at, updated_at

mcp_servers
  id, name (UNIQUE), transport, endpoint, enabled, created_at, updated_at
```

Workflow status values: `draft` | `active` | `running` | `completed` | `failed` | `cancelled`

Task status values: `pending` | `queued` | `running` | `completed` | `failed` | `retrying` | `dead` | `skipped`

---

## License

MIT
