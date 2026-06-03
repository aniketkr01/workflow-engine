package repository

import (
	"context"

	"github.com/aniketkr01/workflow-engine/internal/domain"
	"github.com/google/uuid"
)

type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	Update(ctx context.Context, user *domain.User) error
}

type WorkflowRepository interface {
	Create(ctx context.Context, wf *domain.Workflow) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Workflow, error)
	List(ctx context.Context, ownerID uuid.UUID, limit, offset int) ([]*domain.Workflow, int, error)
	Update(ctx context.Context, wf *domain.Workflow) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type ExecutionRepository interface {
	Create(ctx context.Context, exec *domain.WorkflowExecution) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.WorkflowExecution, error)
	ListByWorkflow(ctx context.Context, workflowID uuid.UUID, limit, offset int) ([]*domain.WorkflowExecution, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.WorkflowStatus) error
	Update(ctx context.Context, exec *domain.WorkflowExecution) error
	ListPending(ctx context.Context) ([]*domain.WorkflowExecution, error)
}

type TaskRepository interface {
	Create(ctx context.Context, task *domain.TaskExecution) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.TaskExecution, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*domain.TaskExecution, error)
	ListByExecution(ctx context.Context, executionID uuid.UUID) ([]*domain.TaskExecution, error)
	Update(ctx context.Context, task *domain.TaskExecution) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.TaskStatus) error
}

type MCPServerRepository interface {
	Create(ctx context.Context, srv *domain.MCPServer) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.MCPServer, error)
	GetByName(ctx context.Context, name string) (*domain.MCPServer, error)
	List(ctx context.Context) ([]*domain.MCPServer, error)
	Update(ctx context.Context, srv *domain.MCPServer) error
	Delete(ctx context.Context, id uuid.UUID) error
}
