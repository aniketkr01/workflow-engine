package engine

import (
	"context"
	"time"

	"github.com/aniketkr01/workflow-engine/internal/domain"
	"github.com/aniketkr01/workflow-engine/internal/logger"
	"github.com/aniketkr01/workflow-engine/internal/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Scheduler periodically checks for delayed/recurring workflows and starts them.
type Scheduler struct {
	workflowRepo  repository.WorkflowRepository
	executionRepo repository.ExecutionRepository
	orchestrator  *Orchestrator
	interval      time.Duration
}

func NewScheduler(
	wfRepo repository.WorkflowRepository,
	execRepo repository.ExecutionRepository,
	orch *Orchestrator,
	interval time.Duration,
) *Scheduler {
	return &Scheduler{
		workflowRepo:  wfRepo,
		executionRepo: execRepo,
		orchestrator:  orch,
		interval:      interval,
	}
}

func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *Scheduler) tick(ctx context.Context) {
	// Recover any running executions whose workers may have crashed.
	pending, err := s.executionRepo.ListPending(ctx)
	if err != nil {
		logger.Error(ctx, "scheduler: list pending executions", zap.Error(err))
		return
	}
	for _, exec := range pending {
		if exec.Status == domain.WorkflowStatusRunning {
			// Re-evaluate in case tasks got stuck.
			if err := s.orchestrator.OnTaskCompleted(ctx, exec.ID, ""); err != nil {
				logger.Error(ctx, "scheduler: recheck execution", zap.Error(err))
			}
		}
	}
}

// TriggerWorkflow immediately starts an execution for the given workflow.
func (s *Scheduler) TriggerWorkflow(ctx context.Context, workflowID uuid.UUID) (*domain.WorkflowExecution, error) {
	return s.orchestrator.StartExecution(ctx, workflowID)
}
