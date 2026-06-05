package grpcserver

import (
	"context"
	"time"

	"github.com/aniketkr01/workflow-engine/internal/engine"
	"github.com/aniketkr01/workflow-engine/internal/logger"
	"github.com/aniketkr01/workflow-engine/internal/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// WorkflowServiceServer implements a minimal gRPC service inline
// (without generated proto stubs, to avoid requiring protoc at build time).
// In a production setup this would be replaced with generated code.

// ExecutionEvent represents a streaming event.
type ExecutionEvent struct {
	ExecutionID string
	EventType   string
	TaskDefID   string
	Status      string
	Timestamp   string
	Message     string
}

// Server wraps the gRPC server setup.
type Server struct {
	grpcServer   *grpc.Server
	orchestrator *engine.Orchestrator
	execRepo     repository.ExecutionRepository
	taskRepo     repository.TaskRepository
}

func NewServer(
	orch *engine.Orchestrator,
	execRepo repository.ExecutionRepository,
	taskRepo repository.TaskRepository,
) *Server {
	s := &Server{
		orchestrator: orch,
		execRepo:     execRepo,
		taskRepo:     taskRepo,
	}

	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(s.unaryInterceptor),
		grpc.StreamInterceptor(s.streamInterceptor),
	}
	s.grpcServer = grpc.NewServer(opts...)
	return s
}

func (s *Server) GRPCServer() *grpc.Server {
	return s.grpcServer
}

// TriggerWorkflow triggers a workflow execution via gRPC.
func (s *Server) TriggerWorkflow(ctx context.Context, workflowID string) (string, error) {
	id, err := uuid.Parse(workflowID)
	if err != nil {
		return "", status.Errorf(codes.InvalidArgument, "invalid workflow id: %v", err)
	}
	exec, err := s.orchestrator.StartExecution(ctx, id)
	if err != nil {
		return "", status.Errorf(codes.Internal, "start execution: %v", err)
	}
	return exec.ID.String(), nil
}

// GetExecutionStatus returns the status of a workflow execution.
func (s *Server) GetExecutionStatus(ctx context.Context, executionID string) (map[string]any, error) {
	id, err := uuid.Parse(executionID)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid execution id: %v", err)
	}

	exec, err := s.execRepo.GetByID(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "execution not found: %v", err)
	}

	tasks, err := s.taskRepo.ListByExecution(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list tasks: %v", err)
	}

	taskStatuses := make([]map[string]any, len(tasks))
	for i, t := range tasks {
		taskStatuses[i] = map[string]any{
			"task_execution_id": t.ID.String(),
			"task_def_id":       t.TaskDefID,
			"task_name":         t.TaskName,
			"status":            string(t.Status),
			"attempt":           t.Attempt,
			"error":             t.Error,
		}
	}

	return map[string]any{
		"execution_id": exec.ID.String(),
		"workflow_id":  exec.WorkflowID.String(),
		"status":       string(exec.Status),
		"started_at":   formatTime(exec.StartedAt),
		"finished_at":  formatTime(exec.FinishedAt),
		"tasks":        taskStatuses,
	}, nil
}

// CancelExecution cancels a running workflow execution.
func (s *Server) CancelExecution(ctx context.Context, executionID string) error {
	id, err := uuid.Parse(executionID)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "invalid execution id: %v", err)
	}
	if err := s.orchestrator.CancelExecution(ctx, id); err != nil {
		return status.Errorf(codes.FailedPrecondition, "cancel execution: %v", err)
	}
	return nil
}

func (s *Server) unaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	start := time.Now()
	resp, err := handler(ctx, req)
	logger.Info(ctx, "grpc unary request",
		zap.String("method", info.FullMethod),
		zap.Duration("latency", time.Since(start)),
		zap.Error(err))
	return resp, err
}

func (s *Server) streamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	start := time.Now()
	err := handler(srv, ss)
	logger.Info(context.Background(), "grpc stream request",
		zap.String("method", info.FullMethod),
		zap.Duration("latency", time.Since(start)),
		zap.Error(err))
	return err
}

func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}
