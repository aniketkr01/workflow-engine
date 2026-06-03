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

type WorkflowRepo struct {
	db *pgxpool.Pool
}

func NewWorkflowRepo(db *pgxpool.Pool) *WorkflowRepo {
	return &WorkflowRepo{db: db}
}

func (r *WorkflowRepo) Create(ctx context.Context, wf *domain.Workflow) error {
	if wf.ID == uuid.Nil {
		wf.ID = uuid.New()
	}
	now := time.Now()
	wf.CreatedAt = now
	wf.UpdatedAt = now
	if wf.Version == 0 {
		wf.Version = 1
	}

	tasksJSON, err := json.Marshal(wf.Tasks)
	if err != nil {
		return fmt.Errorf("marshal tasks: %w", err)
	}
	var scheduleJSON []byte
	if wf.Schedule != nil {
		scheduleJSON, err = json.Marshal(wf.Schedule)
		if err != nil {
			return fmt.Errorf("marshal schedule: %w", err)
		}
	}

	_, err = r.db.Exec(ctx,
		`INSERT INTO workflows (id, name, description, version, owner_id, status, tasks, schedule, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		wf.ID, wf.Name, wf.Description, wf.Version, wf.OwnerID, wf.Status,
		tasksJSON, scheduleJSON, wf.CreatedAt, wf.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create workflow: %w", err)
	}
	return nil
}

func (r *WorkflowRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Workflow, error) {
	var wf domain.Workflow
	var tasksJSON, scheduleJSON []byte

	err := r.db.QueryRow(ctx,
		`SELECT id, name, description, version, owner_id, status, tasks, schedule, created_at, updated_at
		 FROM workflows WHERE id = $1`, id,
	).Scan(&wf.ID, &wf.Name, &wf.Description, &wf.Version, &wf.OwnerID, &wf.Status,
		&tasksJSON, &scheduleJSON, &wf.CreatedAt, &wf.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get workflow: %w", err)
	}

	if err := json.Unmarshal(tasksJSON, &wf.Tasks); err != nil {
		return nil, fmt.Errorf("unmarshal tasks: %w", err)
	}
	if len(scheduleJSON) > 0 {
		wf.Schedule = &domain.Schedule{}
		if err := json.Unmarshal(scheduleJSON, wf.Schedule); err != nil {
			return nil, fmt.Errorf("unmarshal schedule: %w", err)
		}
	}
	return &wf, nil
}

func (r *WorkflowRepo) List(ctx context.Context, ownerID uuid.UUID, limit, offset int) ([]*domain.Workflow, int, error) {
	var total int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM workflows WHERE owner_id = $1`, ownerID,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count workflows: %w", err)
	}

	rows, err := r.db.Query(ctx,
		`SELECT id, name, description, version, owner_id, status, tasks, schedule, created_at, updated_at
		 FROM workflows WHERE owner_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		ownerID, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list workflows: %w", err)
	}
	defer rows.Close()

	var workflows []*domain.Workflow
	for rows.Next() {
		var wf domain.Workflow
		var tasksJSON, scheduleJSON []byte
		if err := rows.Scan(&wf.ID, &wf.Name, &wf.Description, &wf.Version, &wf.OwnerID,
			&wf.Status, &tasksJSON, &scheduleJSON, &wf.CreatedAt, &wf.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan workflow: %w", err)
		}
		if err := json.Unmarshal(tasksJSON, &wf.Tasks); err != nil {
			return nil, 0, err
		}
		if len(scheduleJSON) > 0 {
			wf.Schedule = &domain.Schedule{}
			_ = json.Unmarshal(scheduleJSON, wf.Schedule)
		}
		workflows = append(workflows, &wf)
	}
	return workflows, total, nil
}

func (r *WorkflowRepo) Update(ctx context.Context, wf *domain.Workflow) error {
	wf.UpdatedAt = time.Now()
	tasksJSON, err := json.Marshal(wf.Tasks)
	if err != nil {
		return fmt.Errorf("marshal tasks: %w", err)
	}
	var scheduleJSON []byte
	if wf.Schedule != nil {
		scheduleJSON, err = json.Marshal(wf.Schedule)
		if err != nil {
			return fmt.Errorf("marshal schedule: %w", err)
		}
	}
	_, err = r.db.Exec(ctx,
		`UPDATE workflows SET name=$1, description=$2, version=$3, status=$4, tasks=$5, schedule=$6, updated_at=$7
		 WHERE id=$8`,
		wf.Name, wf.Description, wf.Version, wf.Status, tasksJSON, scheduleJSON, wf.UpdatedAt, wf.ID,
	)
	if err != nil {
		return fmt.Errorf("update workflow: %w", err)
	}
	return nil
}

func (r *WorkflowRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM workflows WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete workflow: %w", err)
	}
	return nil
}
