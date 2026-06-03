package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aniketkr01/workflow-engine/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ExecutionRepo struct {
	db *pgxpool.Pool
}

func NewExecutionRepo(db *pgxpool.Pool) *ExecutionRepo {
	return &ExecutionRepo{db: db}
}

func (r *ExecutionRepo) Create(ctx context.Context, exec *domain.WorkflowExecution) error {
	if exec.ID == uuid.Nil {
		exec.ID = uuid.New()
	}
	now := time.Now()
	exec.CreatedAt = now
	exec.UpdatedAt = now

	outputsJSON, err := json.Marshal(exec.Outputs)
	if err != nil {
		return fmt.Errorf("marshal outputs: %w", err)
	}

	_, err = r.db.Exec(ctx,
		`INSERT INTO workflow_executions (id, workflow_id, version, status, started_at, finished_at, outputs, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		exec.ID, exec.WorkflowID, exec.Version, exec.Status,
		exec.StartedAt, exec.FinishedAt, outputsJSON, exec.CreatedAt, exec.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create execution: %w", err)
	}
	return nil
}

func (r *ExecutionRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.WorkflowExecution, error) {
	var exec domain.WorkflowExecution
	var outputsJSON []byte
	err := r.db.QueryRow(ctx,
		`SELECT id, workflow_id, version, status, started_at, finished_at, outputs, created_at, updated_at
		 FROM workflow_executions WHERE id = $1`, id,
	).Scan(&exec.ID, &exec.WorkflowID, &exec.Version, &exec.Status,
		&exec.StartedAt, &exec.FinishedAt, &outputsJSON, &exec.CreatedAt, &exec.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get execution: %w", err)
	}
	if len(outputsJSON) > 0 {
		_ = json.Unmarshal(outputsJSON, &exec.Outputs)
	}
	return &exec, nil
}

func (r *ExecutionRepo) ListByWorkflow(ctx context.Context, workflowID uuid.UUID, limit, offset int) ([]*domain.WorkflowExecution, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, workflow_id, version, status, started_at, finished_at, outputs, created_at, updated_at
		 FROM workflow_executions WHERE workflow_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		workflowID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list executions: %w", err)
	}
	defer rows.Close()

	var execs []*domain.WorkflowExecution
	for rows.Next() {
		var exec domain.WorkflowExecution
		var outputsJSON []byte
		if err := rows.Scan(&exec.ID, &exec.WorkflowID, &exec.Version, &exec.Status,
			&exec.StartedAt, &exec.FinishedAt, &outputsJSON, &exec.CreatedAt, &exec.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan execution: %w", err)
		}
		if len(outputsJSON) > 0 {
			_ = json.Unmarshal(outputsJSON, &exec.Outputs)
		}
		execs = append(execs, &exec)
	}
	return execs, nil
}

func (r *ExecutionRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.WorkflowStatus) error {
	_, err := r.db.Exec(ctx,
		`UPDATE workflow_executions SET status=$1, updated_at=$2 WHERE id=$3`,
		status, time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("update execution status: %w", err)
	}
	return nil
}

func (r *ExecutionRepo) Update(ctx context.Context, exec *domain.WorkflowExecution) error {
	exec.UpdatedAt = time.Now()
	outputsJSON, err := json.Marshal(exec.Outputs)
	if err != nil {
		return fmt.Errorf("marshal outputs: %w", err)
	}
	_, err = r.db.Exec(ctx,
		`UPDATE workflow_executions SET status=$1, started_at=$2, finished_at=$3, outputs=$4, updated_at=$5
		 WHERE id=$6`,
		exec.Status, exec.StartedAt, exec.FinishedAt, outputsJSON, exec.UpdatedAt, exec.ID,
	)
	if err != nil {
		return fmt.Errorf("update execution: %w", err)
	}
	return nil
}

func (r *ExecutionRepo) ListPending(ctx context.Context) ([]*domain.WorkflowExecution, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, workflow_id, version, status, started_at, finished_at, outputs, created_at, updated_at
		 FROM workflow_executions WHERE status IN ('pending', 'running') ORDER BY created_at ASC LIMIT 100`,
	)
	if err != nil {
		return nil, fmt.Errorf("list pending executions: %w", err)
	}
	defer rows.Close()

	var execs []*domain.WorkflowExecution
	for rows.Next() {
		var exec domain.WorkflowExecution
		var outputsJSON []byte
		if err := rows.Scan(&exec.ID, &exec.WorkflowID, &exec.Version, &exec.Status,
			&exec.StartedAt, &exec.FinishedAt, &outputsJSON, &exec.CreatedAt, &exec.UpdatedAt); err != nil {
			return nil, err
		}
		if len(outputsJSON) > 0 {
			_ = json.Unmarshal(outputsJSON, &exec.Outputs)
		}
		execs = append(execs, &exec)
	}
	return execs, nil
}
