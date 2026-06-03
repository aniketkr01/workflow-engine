package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aniketkr01/workflow-engine/internal/config"
	"github.com/aniketkr01/workflow-engine/internal/domain"
	"github.com/aniketkr01/workflow-engine/internal/engine"
	"github.com/aniketkr01/workflow-engine/internal/logger"
	"github.com/aniketkr01/workflow-engine/internal/mcp"
	"github.com/aniketkr01/workflow-engine/internal/queue"
	"github.com/aniketkr01/workflow-engine/internal/repository"
	"github.com/aniketkr01/workflow-engine/internal/telemetry"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Pool is a pool of concurrent workers that consume tasks from the queue.
type Pool struct {
	id           string
	cfg          config.WorkerConfig
	queue        *queue.Queue
	taskRepo     repository.TaskRepository
	orchestrator *engine.Orchestrator
	mcpRegistry  *mcp.Registry
	metrics      *telemetry.Metrics
	wg           sync.WaitGroup
}

func NewPool(
	id string,
	cfg config.WorkerConfig,
	q *queue.Queue,
	taskRepo repository.TaskRepository,
	orch *engine.Orchestrator,
	mcpRegistry *mcp.Registry,
	metrics *telemetry.Metrics,
) *Pool {
	return &Pool{
		id:           id,
		cfg:          cfg,
		queue:        q,
		taskRepo:     taskRepo,
		orchestrator: orch,
		mcpRegistry:  mcpRegistry,
		metrics:      metrics,
	}
}

// Run starts the worker pool. Blocks until ctx is cancelled.
func (p *Pool) Run(ctx context.Context) {
	sem := make(chan struct{}, p.cfg.Concurrency)
	for {
		select {
		case <-ctx.Done():
			p.wg.Wait()
			return
		default:
		}

		msgs, err := p.queue.Consume(ctx, p.id, p.cfg.Concurrency, p.cfg.PollInterval)
		if err != nil {
			logger.Error(ctx, "worker: consume from queue", zap.Error(err))
			time.Sleep(time.Second)
			continue
		}

		for i := range msgs {
			msg := msgs[i]
			sem <- struct{}{}
			p.wg.Add(1)
			go func(m redis.XMessage) {
				defer func() { <-sem; p.wg.Done() }()
				p.handleMessage(ctx, m)
			}(msg)
		}
	}
}

func (p *Pool) handleMessage(ctx context.Context, msg redis.XMessage) {
	tm, err := queue.ParseMessage(msg)
	if err != nil {
		logger.Error(ctx, "worker: parse message", zap.Error(err), zap.String("msg_id", msg.ID))
		_ = p.queue.Ack(ctx, msg.ID)
		return
	}
	p.processTask(ctx, msg.ID, tm)
}

// processTask executes a single task.
func (p *Pool) processTask(ctx context.Context, msgID string, tm *queue.TaskMessage) {
	taskExecID, err := uuid.Parse(tm.TaskExecutionID)
	if err != nil {
		logger.Error(ctx, "invalid task execution id",
			zap.String("task_execution_id", tm.TaskExecutionID),
			zap.String("task_def_id", tm.TaskDefID),
			zap.Int("attempt", tm.Attempt),
			zap.Error(err))
		_ = p.queue.Ack(ctx, msgID)
		return
	}

	task, err := p.taskRepo.GetByID(ctx, taskExecID)
	if err != nil {
		logger.Error(ctx, "get task execution",
			zap.String("task_execution_id", tm.TaskExecutionID),
			zap.String("task_def_id", tm.TaskDefID),
			zap.Int("attempt", tm.Attempt),
			zap.Error(err))
		_ = p.queue.Ack(ctx, msgID)
		return
	}

	// Idempotency: skip if already finished.
	if task.Status == domain.TaskStatusCompleted || task.Status == domain.TaskStatusDead {
		logger.Info(ctx, "task already finished, skipping",
			zap.String("task_execution_id", tm.TaskExecutionID),
			zap.String("task_def_id", tm.TaskDefID),
			zap.Int("attempt", tm.Attempt),
			zap.String("status", string(task.Status)))
		_ = p.queue.Ack(ctx, msgID)
		return
	}

	now := time.Now()
	task.Status = domain.TaskStatusRunning
	task.StartedAt = &now
	task.WorkerID = p.id
	if err := p.taskRepo.Update(ctx, task); err != nil {
		logger.Error(ctx, "update task to running",
			zap.String("task_execution_id", tm.TaskExecutionID),
			zap.String("task_def_id", tm.TaskDefID),
			zap.Int("attempt", tm.Attempt),
			zap.Error(err))
	}

	p.metrics.TasksRunning.Inc()
	defer p.metrics.TasksRunning.Dec()

	timeout := time.Duration(tm.TimeoutSec) * time.Second
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	output, execErr := p.executeTask(execCtx, tm)

	finishedAt := time.Now()
	task.FinishedAt = &finishedAt

	if execErr != nil {
		logger.Error(ctx, "task execution failed",
			zap.String("task_execution_id", tm.TaskExecutionID),
			zap.String("task_def_id", tm.TaskDefID),
			zap.Int("attempt", tm.Attempt),
			zap.Error(execErr))
		task.Error = execErr.Error()
		task.Status = domain.TaskStatusFailed
		if err := p.taskRepo.Update(ctx, task); err != nil {
			logger.Error(ctx, "update failed task",
				zap.String("task_execution_id", tm.TaskExecutionID),
				zap.String("task_def_id", tm.TaskDefID),
				zap.Int("attempt", tm.Attempt),
				zap.Error(err))
		}
		p.metrics.TasksFailed.Inc()
		_ = p.queue.Ack(ctx, msgID)

		_ = p.orchestrator.OnTaskFailed(ctx, taskExecID, execErr.Error())
		return
	}

	task.Output = output
	task.Status = domain.TaskStatusCompleted
	if err := p.taskRepo.Update(ctx, task); err != nil {
		logger.Error(ctx, "update completed task",
			zap.String("task_execution_id", tm.TaskExecutionID),
			zap.String("task_def_id", tm.TaskDefID),
			zap.Int("attempt", tm.Attempt),
			zap.Error(err))
	}
	p.metrics.TasksCompleted.Inc()
	_ = p.queue.Ack(ctx, msgID)

	execID, _ := uuid.Parse(tm.ExecutionID)
	if err := p.orchestrator.OnTaskCompleted(ctx, execID, tm.TaskDefID); err != nil {
		logger.Error(ctx, "notify orchestrator of completion",
			zap.String("task_execution_id", tm.TaskExecutionID),
			zap.String("task_def_id", tm.TaskDefID),
			zap.Int("attempt", tm.Attempt),
			zap.Error(err))
	}
}

func (p *Pool) executeTask(ctx context.Context, tm *queue.TaskMessage) (map[string]any, error) {
	taskType, _ := tm.Input["__type"].(string)
	switch taskType {
	case "mcp_tool":
		return p.executeMCPTool(ctx, tm)
	case "http":
		return p.executeHTTPTask(ctx, tm)
	default:
		// Default to MCP tool if server/tool specified.
		if _, ok := tm.Input["__mcp_server"]; ok {
			return p.executeMCPTool(ctx, tm)
		}
		return map[string]any{"status": "ok"}, nil
	}
}

func (p *Pool) executeMCPTool(ctx context.Context, tm *queue.TaskMessage) (map[string]any, error) {
	serverName, _ := tm.Input["__mcp_server"].(string)
	toolName, _ := tm.Input["__tool_name"].(string)

	if serverName == "" || toolName == "" {
		return nil, fmt.Errorf("missing __mcp_server or __tool_name in task input")
	}

	client, _, err := p.mcpRegistry.GetTool(serverName, toolName)
	if err != nil {
		return nil, fmt.Errorf("resolve mcp tool: %w", err)
	}

	args := make(map[string]any)
	for k, v := range tm.Input {
		if k != "__type" && k != "__mcp_server" && k != "__tool_name" {
			args[k] = v
		}
	}

	result, err := client.CallTool(ctx, toolName, args)
	if err != nil {
		return nil, fmt.Errorf("call mcp tool: %w", err)
	}
	if result.IsError {
		if len(result.Content) > 0 {
			return nil, fmt.Errorf("mcp tool error: %s", result.Content[0].Text)
		}
		return nil, fmt.Errorf("mcp tool returned error")
	}

	output := make(map[string]any)
	for i, c := range result.Content {
		output[fmt.Sprintf("content_%d", i)] = c.Text
	}
	if len(result.Content) == 1 {
		output["result"] = result.Content[0].Text
	}
	return output, nil
}

func (p *Pool) executeHTTPTask(ctx context.Context, tm *queue.TaskMessage) (map[string]any, error) {
	return map[string]any{"status": "ok", "task_def_id": tm.TaskDefID}, nil
}
