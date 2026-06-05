package domain

import (
	"time"

	"github.com/google/uuid"
)

type WorkflowStatus string

const (
	WorkflowStatusDraft     WorkflowStatus = "draft"
	WorkflowStatusActive    WorkflowStatus = "active"
	WorkflowStatusRunning   WorkflowStatus = "running"
	WorkflowStatusCompleted WorkflowStatus = "completed"
	WorkflowStatusFailed    WorkflowStatus = "failed"
	WorkflowStatusCancelled WorkflowStatus = "cancelled"
)

type ScheduleType string

const (
	ScheduleImmediate ScheduleType = "immediate"
	ScheduleDelayed   ScheduleType = "delayed"
	ScheduleRecurring ScheduleType = "recurring"
)

// Workflow is the top-level definition of a workflow.
type Workflow struct {
	ID          uuid.UUID      `json:"id" db:"id"`
	Name        string         `json:"name" db:"name"`
	Description string         `json:"description" db:"description"`
	Version     int            `json:"version" db:"version"`
	OwnerID     uuid.UUID      `json:"owner_id" db:"owner_id"`
	Status      WorkflowStatus `json:"status" db:"status"`
	Tasks       []TaskDef      `json:"tasks" db:"tasks"` // stored as JSONB
	Schedule    *Schedule      `json:"schedule,omitempty" db:"schedule"`
	CreatedAt   time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at" db:"updated_at"`
}

// TaskDef is a node in the DAG.
type TaskDef struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Type         string            `json:"type"` // "mcp_tool" | "http" | "script"
	MCPServer    string            `json:"mcp_server,omitempty"`
	ToolName     string            `json:"tool_name,omitempty"`
	Parameters   map[string]any    `json:"parameters,omitempty"`
	Dependencies []string          `json:"dependencies,omitempty"` // task IDs
	RetryPolicy  RetryPolicy       `json:"retry_policy"`
	TimeoutSec   int               `json:"timeout_sec"`
	InputMapping map[string]string `json:"input_mapping,omitempty"` // maps output keys of deps to input params
}

type RetryPolicy struct {
	MaxAttempts int `json:"max_attempts"`
	BackoffSec  int `json:"backoff_sec"`
}

type Schedule struct {
	Type     ScheduleType `json:"type"`
	RunAt    *time.Time   `json:"run_at,omitempty"`    // for delayed
	CronExpr string       `json:"cron_expr,omitempty"` // for recurring
}

// WorkflowExecution is a single run of a Workflow.
type WorkflowExecution struct {
	ID         uuid.UUID      `json:"id" db:"id"`
	WorkflowID uuid.UUID      `json:"workflow_id" db:"workflow_id"`
	Version    int            `json:"version" db:"version"`
	Status     WorkflowStatus `json:"status" db:"status"`
	StartedAt  *time.Time     `json:"started_at,omitempty" db:"started_at"`
	FinishedAt *time.Time     `json:"finished_at,omitempty" db:"finished_at"`
	CreatedAt  time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at" db:"updated_at"`
	// Outputs aggregated after completion
	Outputs map[string]any `json:"outputs,omitempty" db:"outputs"`
}
