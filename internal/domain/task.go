package domain

import (
	"time"

	"github.com/google/uuid"
)

type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusQueued    TaskStatus = "queued"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusRetrying  TaskStatus = "retrying"
	TaskStatusDead      TaskStatus = "dead" // exceeded retries
	TaskStatusSkipped   TaskStatus = "skipped"
)

// TaskExecution is the runtime record of a single task within a workflow execution.
type TaskExecution struct {
	ID                uuid.UUID      `json:"id" db:"id"`
	ExecutionID       uuid.UUID      `json:"execution_id" db:"execution_id"`
	WorkflowID        uuid.UUID      `json:"workflow_id" db:"workflow_id"`
	TaskDefID         string         `json:"task_def_id" db:"task_def_id"`
	TaskName          string         `json:"task_name" db:"task_name"`
	Status            TaskStatus     `json:"status" db:"status"`
	Attempt           int            `json:"attempt" db:"attempt"`
	MaxAttempts       int            `json:"max_attempts" db:"max_attempts"`
	Input             map[string]any `json:"input,omitempty" db:"input"`
	Output            map[string]any `json:"output,omitempty" db:"output"`
	Error             string         `json:"error,omitempty" db:"error"`
	StartedAt         *time.Time     `json:"started_at,omitempty" db:"started_at"`
	FinishedAt        *time.Time     `json:"finished_at,omitempty" db:"finished_at"`
	CreatedAt         time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at" db:"updated_at"`
	IdempotencyKey    string         `json:"idempotency_key" db:"idempotency_key"`
	WorkerID          string         `json:"worker_id,omitempty" db:"worker_id"`
}

// MCPServer represents a registered MCP server.
type MCPServer struct {
	ID        uuid.UUID `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Transport string    `json:"transport" db:"transport"` // "http" | "stdio"
	Endpoint  string    `json:"endpoint" db:"endpoint"`   // URL for http, command for stdio
	Enabled   bool      `json:"enabled" db:"enabled"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// MCPTool is a discovered tool from an MCP server.
type MCPTool struct {
	ServerID    uuid.UUID      `json:"server_id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}
