package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aniketkr01/workflow-engine/internal/domain"
	"github.com/aniketkr01/workflow-engine/internal/logger"
	"github.com/aniketkr01/workflow-engine/internal/queue"
	"github.com/aniketkr01/workflow-engine/internal/repository"
	"github.com/aniketkr01/workflow-engine/internal/telemetry"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Orchestrator drives workflow execution by resolving DAG dependencies
// and dispatching ready tasks to the queue.
type Orchestrator struct {
	workflowRepo  repository.WorkflowRepository
	executionRepo repository.ExecutionRepository
	taskRepo      repository.TaskRepository
	queue         *queue.Queue
	metrics       *telemetry.Metrics
	mu            sync.Mutex
}

func NewOrchestrator(
	wfRepo repository.WorkflowRepository,
	execRepo repository.ExecutionRepository,
	taskRepo repository.TaskRepository,
	q *queue.Queue,
	metrics *telemetry.Metrics,
) *Orchestrator {
	return &Orchestrator{
		workflowRepo:  wfRepo,
		executionRepo: execRepo,
		taskRepo:      taskRepo,
		queue:         q,
		metrics:       metrics,
	}
}

// StartExecution creates a WorkflowExecution and dispatches initial tasks.
func (o *Orchestrator) StartExecution(ctx context.Context, workflowID uuid.UUID) (*domain.WorkflowExecution, error) {
	wf, err := o.workflowRepo.GetByID(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("get workflow: %w", err)
	}

	dag, err := BuildDAG(wf.Tasks)
	if err != nil {
		return nil, fmt.Errorf("build dag: %w", err)
	}

	now := time.Now()
	exec := &domain.WorkflowExecution{
		WorkflowID: workflowID,
		Version:    wf.Version,
		Status:     domain.WorkflowStatusRunning,
		StartedAt:  &now,
	}
	if err := o.executionRepo.Create(ctx, exec); err != nil {
		return nil, fmt.Errorf("create execution: %w", err)
	}

	o.metrics.WorkflowStarted.Inc()

	// Dispatch tasks that have no dependencies.
	initialTasks := dag.ReadyTasks(map[string]bool{})
	for _, taskID := range initialTasks {
		taskDef, _ := dag.GetTask(taskID)
		if err := o.dispatchTask(ctx, exec, taskDef, nil); err != nil {
			logger.Error(ctx, "failed to dispatch initial task", zap.Any("task_id", taskID), zap.Error(err))
		}
	}

	return exec, nil
}

// OnTaskCompleted handles task completion and dispatches newly unblocked tasks.
func (o *Orchestrator) OnTaskCompleted(ctx context.Context, executionID uuid.UUID, completedTaskDefID string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	exec, err := o.executionRepo.GetByID(ctx, executionID)
	if err != nil {
		return fmt.Errorf("get execution: %w", err)
	}

	if exec.Status != domain.WorkflowStatusRunning {
		logger.Info(ctx, "execution not running, skipping completion handling",
			zap.String("execution_id", exec.ID.String()),
			zap.String("status", string(exec.Status)),
			zap.String("completed_task_def_id", completedTaskDefID))
		return nil
	}

	wf, err := o.workflowRepo.GetByID(ctx, exec.WorkflowID)
	if err != nil {
		return fmt.Errorf("get workflow: %w", err)
	}

	dag, err := BuildDAG(wf.Tasks)
	if err != nil {
		return fmt.Errorf("build dag: %w", err)
	}

	allTasks, err := o.taskRepo.ListByExecution(ctx, executionID)
	if err != nil {
		return fmt.Errorf("list tasks: %w", err)
	}

	completed := make(map[string]bool)
	allCompleted := true
	hasFailed := false
	for _, t := range allTasks {
		if t.Status == domain.TaskStatusCompleted {
			completed[t.TaskDefID] = true
		} else if t.Status == domain.TaskStatusDead || t.Status == domain.TaskStatusFailed {
			hasFailed = true
			allCompleted = false
			break
		} else {
			allCompleted = false
		}
	}

	if hasFailed {
		return o.finalizeExecution(ctx, exec, domain.WorkflowStatusFailed)
	}

	// All tasks completed?
	if allCompleted && len(allTasks) == len(wf.Tasks) {
		o.metrics.WorkflowCompleted.Inc()
		return o.finalizeExecution(ctx, exec, domain.WorkflowStatusCompleted)
	}

	// Dispatch newly ready tasks.
	readyTaskIDs := dag.ReadyTasks(completed)
	for _, taskID := range readyTaskIDs {
		// Skip if already dispatched.
		alreadyQueued := false
		for _, t := range allTasks {
			if t.TaskDefID == taskID {
				alreadyQueued = true
				break
			}
		}
		if alreadyQueued {
			continue
		}

		taskDef, _ := dag.GetTask(taskID)
		outputs := o.collectDependencyOutputs(allTasks, taskDef)
		if err := o.dispatchTask(ctx, exec, taskDef, outputs); err != nil {
			logger.Error(ctx, "failed to dispatch task", zap.Any("task_id", taskID), zap.Error(err))
		}
	}
	return nil
}

// OnTaskFailed handles task failure, retrying if allowed or failing the workflow.
func (o *Orchestrator) OnTaskFailed(ctx context.Context, taskExecID uuid.UUID, errMsg string) error {
	task, err := o.taskRepo.GetByID(ctx, taskExecID)
	if err != nil {
		return fmt.Errorf("get task: %w", err)
	}

	task.Error = errMsg

	if task.Attempt < task.MaxAttempts {
		task.Status = domain.TaskStatusRetrying
		task.Attempt++
		if err := o.taskRepo.Update(ctx, task); err != nil {
			return fmt.Errorf("update task for retry: %w", err)
		}
		o.metrics.TaskRetries.Inc()

		exec, err := o.executionRepo.GetByID(ctx, task.ExecutionID)
		if err != nil {
			return fmt.Errorf("get execution: %w", err)
		}
		wf, err := o.workflowRepo.GetByID(ctx, exec.WorkflowID)
		if err != nil {
			return fmt.Errorf("get workflow: %w", err)
		}
		var taskDef *domain.TaskDef
		for i := range wf.Tasks {
			if wf.Tasks[i].ID == task.TaskDefID {
				taskDef = &wf.Tasks[i]
				break
			}
		}
		if taskDef == nil {
			return fmt.Errorf("task def not found: %s", task.TaskDefID)
		}

		msg := queue.TaskMessage{
			TaskExecutionID: task.ID.String(),
			ExecutionID:     task.ExecutionID.String(),
			WorkflowID:      task.WorkflowID.String(),
			TaskDefID:       task.TaskDefID,
			Attempt:         task.Attempt,
			Input:           task.Input,
			TimeoutSec:      taskDef.TimeoutSec,
			IdempotencyKey:  fmt.Sprintf("%s-%d", task.IdempotencyKey[:len(task.IdempotencyKey)-1], task.Attempt),
		}
		return o.queue.Enqueue(ctx, msg)
	}

	// Exceeded retries -> dead letter
	task.Status = domain.TaskStatusDead
	if err := o.taskRepo.Update(ctx, task); err != nil {
		return fmt.Errorf("update task dead: %w", err)
	}
	o.metrics.TasksDead.Inc()

	dlqMsg := queue.TaskMessage{
		TaskExecutionID: task.ID.String(),
		ExecutionID:     task.ExecutionID.String(),
		WorkflowID:      task.WorkflowID.String(),
		TaskDefID:       task.TaskDefID,
		Attempt:         task.Attempt,
		Input:           task.Input,
		IdempotencyKey:  task.IdempotencyKey,
	}
	_ = o.queue.MoveToDLQ(ctx, dlqMsg)

	return o.OnTaskCompleted(ctx, task.ExecutionID, task.TaskDefID) // triggers failure check
}

// CancelExecution cancels a running workflow execution.
func (o *Orchestrator) CancelExecution(ctx context.Context, executionID uuid.UUID) error {
	exec, err := o.executionRepo.GetByID(ctx, executionID)
	if err != nil {
		return fmt.Errorf("get execution: %w", err)
	}
	if exec.Status != domain.WorkflowStatusRunning {
		return fmt.Errorf("execution is not running (status: %s)", exec.Status)
	}
	return o.finalizeExecution(ctx, exec, domain.WorkflowStatusCancelled)
}

// dispatchTask persists a TaskExecution and enqueues it.
func (o *Orchestrator) dispatchTask(ctx context.Context, exec *domain.WorkflowExecution, taskDef *domain.TaskDef, depOutputs map[string]any) error {
	maxAttempts := taskDef.RetryPolicy.MaxAttempts
	if maxAttempts == 0 {
		maxAttempts = 3
	}

	input := make(map[string]any)
	for k, v := range taskDef.Parameters {
		input[k] = v
	}
	for k, v := range depOutputs {
		input[k] = v
	}

	idempotencyKey := fmt.Sprintf("%s-%s-0", exec.ID.String(), taskDef.ID)

	task := &domain.TaskExecution{
		ExecutionID:    exec.ID,
		WorkflowID:     exec.WorkflowID,
		TaskDefID:      taskDef.ID,
		TaskName:       taskDef.Name,
		Status:         domain.TaskStatusQueued,
		Attempt:        0,
		MaxAttempts:    maxAttempts,
		Input:          input,
		IdempotencyKey: idempotencyKey,
	}

	// Idempotency check
	if existing, _ := o.taskRepo.GetByIdempotencyKey(ctx, idempotencyKey); existing != nil {
		return nil // already dispatched
	}

	if err := o.taskRepo.Create(ctx, task); err != nil {
		return fmt.Errorf("create task execution: %w", err)
	}

	timeout := taskDef.TimeoutSec
	if timeout == 0 {
		timeout = 300
	}

	msg := queue.TaskMessage{
		TaskExecutionID: task.ID.String(),
		ExecutionID:     exec.ID.String(),
		WorkflowID:      exec.WorkflowID.String(),
		TaskDefID:       taskDef.ID,
		Attempt:         0,
		Input:           input,
		TimeoutSec:      timeout,
		IdempotencyKey:  idempotencyKey,
	}

	o.metrics.TasksDispatched.Inc()
	return o.queue.Enqueue(ctx, msg)
}

func (o *Orchestrator) finalizeExecution(ctx context.Context, exec *domain.WorkflowExecution, status domain.WorkflowStatus) error {
	now := time.Now()
	exec.Status = status
	exec.FinishedAt = &now
	return o.executionRepo.Update(ctx, exec)
}

func (o *Orchestrator) collectDependencyOutputs(tasks []*domain.TaskExecution, taskDef *domain.TaskDef) map[string]any {
	outputs := make(map[string]any)
	depSet := make(map[string]bool)
	for _, dep := range taskDef.Dependencies {
		depSet[dep] = true
	}
	for _, t := range tasks {
		if depSet[t.TaskDefID] && t.Status == domain.TaskStatusCompleted {
			for k, v := range t.Output {
				// Apply input mapping if defined.
				mappedKey := k
				if taskDef.InputMapping != nil {
					if mk, ok := taskDef.InputMapping[t.TaskDefID+"."+k]; ok {
						mappedKey = mk
					}
				}
				outputs[mappedKey] = v
			}
		}
	}
	return outputs
}
