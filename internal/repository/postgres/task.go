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

type TaskRepo struct {
	db *pgxpool.Pool
}

func NewTaskRepo(db *pgxpool.Pool) *TaskRepo {
	return &TaskRepo{db: db}
}

func (r *TaskRepo) Create(ctx context.Context, task *domain.TaskExecution) error {
	if task.ID == uuid.Nil {
		task.ID = uuid.New()
	}
	now := time.Now()
	task.CreatedAt = now
	task.UpdatedAt = now

	inputJSON, _ := json.Marshal(task.Input)
	outputJSON, _ := json.Marshal(task.Output)

	_, err := r.db.Exec(ctx,
		`INSERT INTO task_executions
		 (id, execution_id, workflow_id, task_def_id, task_name, status, attempt, max_attempts,
		  input, output, error, started_at, finished_at, idempotency_key, worker_id, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
		task.ID, task.ExecutionID, task.WorkflowID, task.TaskDefID, task.TaskName,
		task.Status, task.Attempt, task.MaxAttempts,
		inputJSON, outputJSON, task.Error, task.StartedAt, task.FinishedAt,
		task.IdempotencyKey, task.WorkerID, task.CreatedAt, task.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create task execution: %w", err)
	}
	return nil
}

func (r *TaskRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.TaskExecution, error) {
	var task domain.TaskExecution
	var inputJSON, outputJSON []byte
	err := r.db.QueryRow(ctx,
		`SELECT id, execution_id, workflow_id, task_def_id, task_name, status, attempt, max_attempts,
		        input, output, error, started_at, finished_at, idempotency_key, worker_id, created_at, updated_at
		 FROM task_executions WHERE id = $1`, id,
	).Scan(&task.ID, &task.ExecutionID, &task.WorkflowID, &task.TaskDefID, &task.TaskName,
		&task.Status, &task.Attempt, &task.MaxAttempts,
		&inputJSON, &outputJSON, &task.Error, &task.StartedAt, &task.FinishedAt,
		&task.IdempotencyKey, &task.WorkerID, &task.CreatedAt, &task.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get task execution: %w", err)
	}
	_ = json.Unmarshal(inputJSON, &task.Input)
	_ = json.Unmarshal(outputJSON, &task.Output)
	return &task, nil
}

func (r *TaskRepo) GetByIdempotencyKey(ctx context.Context, key string) (*domain.TaskExecution, error) {
	var task domain.TaskExecution
	var inputJSON, outputJSON []byte
	err := r.db.QueryRow(ctx,
		`SELECT id, execution_id, workflow_id, task_def_id, task_name, status, attempt, max_attempts,
		        input, output, error, started_at, finished_at, idempotency_key, worker_id, created_at, updated_at
		 FROM task_executions WHERE idempotency_key = $1`, key,
	).Scan(&task.ID, &task.ExecutionID, &task.WorkflowID, &task.TaskDefID, &task.TaskName,
		&task.Status, &task.Attempt, &task.MaxAttempts,
		&inputJSON, &outputJSON, &task.Error, &task.StartedAt, &task.FinishedAt,
		&task.IdempotencyKey, &task.WorkerID, &task.CreatedAt, &task.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get task by idempotency key: %w", err)
	}
	_ = json.Unmarshal(inputJSON, &task.Input)
	_ = json.Unmarshal(outputJSON, &task.Output)
	return &task, nil
}

func (r *TaskRepo) ListByExecution(ctx context.Context, executionID uuid.UUID) ([]*domain.TaskExecution, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, execution_id, workflow_id, task_def_id, task_name, status, attempt, max_attempts,
		        input, output, error, started_at, finished_at, idempotency_key, worker_id, created_at, updated_at
		 FROM task_executions WHERE execution_id = $1 ORDER BY created_at ASC`, executionID,
	)
	if err != nil {
		return nil, fmt.Errorf("list task executions: %w", err)
	}
	defer rows.Close()

	var tasks []*domain.TaskExecution
	for rows.Next() {
		var task domain.TaskExecution
		var inputJSON, outputJSON []byte
		if err := rows.Scan(&task.ID, &task.ExecutionID, &task.WorkflowID, &task.TaskDefID, &task.TaskName,
			&task.Status, &task.Attempt, &task.MaxAttempts,
			&inputJSON, &outputJSON, &task.Error, &task.StartedAt, &task.FinishedAt,
			&task.IdempotencyKey, &task.WorkerID, &task.CreatedAt, &task.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan task execution: %w", err)
		}
		_ = json.Unmarshal(inputJSON, &task.Input)
		_ = json.Unmarshal(outputJSON, &task.Output)
		tasks = append(tasks, &task)
	}
	return tasks, nil
}

func (r *TaskRepo) Update(ctx context.Context, task *domain.TaskExecution) error {
	task.UpdatedAt = time.Now()
	inputJSON, _ := json.Marshal(task.Input)
	outputJSON, _ := json.Marshal(task.Output)
	_, err := r.db.Exec(ctx,
		`UPDATE task_executions SET status=$1, attempt=$2, input=$3, output=$4, error=$5,
		        started_at=$6, finished_at=$7, worker_id=$8, updated_at=$9
		 WHERE id=$10`,
		task.Status, task.Attempt, inputJSON, outputJSON, task.Error,
		task.StartedAt, task.FinishedAt, task.WorkerID, task.UpdatedAt, task.ID,
	)
	if err != nil {
		return fmt.Errorf("update task execution: %w", err)
	}
	return nil
}

func (r *TaskRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.TaskStatus) error {
	_, err := r.db.Exec(ctx,
		`UPDATE task_executions SET status=$1, updated_at=$2 WHERE id=$3`,
		status, time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("update task status: %w", err)
	}
	return nil
}
