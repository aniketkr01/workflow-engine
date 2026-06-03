package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/aniketkr01/workflow-engine/internal/config"
	"github.com/aniketkr01/workflow-engine/internal/engine"
	"github.com/aniketkr01/workflow-engine/internal/logger"
	"github.com/aniketkr01/workflow-engine/internal/mcp"
	"github.com/aniketkr01/workflow-engine/internal/queue"
	"github.com/aniketkr01/workflow-engine/internal/repository/postgres"
	"github.com/aniketkr01/workflow-engine/internal/telemetry"
	"github.com/aniketkr01/workflow-engine/internal/worker"
)

func main() {
	cfg := config.Load()

	if err := logger.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize stdouttrace exporter: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Tracing.
	if cfg.Telemetry.TracingEnabled {
		shutdown, err := telemetry.InitTracing(ctx, cfg.Telemetry.ServiceName+"-worker", cfg.Telemetry.OTELEndpoint)
		if err != nil {
			logger.Warn(ctx, "tracing init failed", zap.Error(err))
		} else {
			defer func() {
				shutdownCtx, c := context.WithTimeout(context.Background(), 5*time.Second)
				defer c()
				_ = shutdown(shutdownCtx)
			}()
		}
	}

	// Database.
	db, err := postgres.NewPool(ctx, cfg.Database)
	if err != nil {
		logger.Fatal(ctx, "connect to database", zap.Error(err))
	}
	defer db.Close()

	// Redis.
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer rdb.Close()

	// Repositories.
	workflowRepo := postgres.NewWorkflowRepo(db)
	executionRepo := postgres.NewExecutionRepo(db)
	taskRepo := postgres.NewTaskRepo(db)
	mcpServerRepo := postgres.NewMCPServerRepo(db)

	// Metrics.
	metrics := telemetry.NewMetrics()

	// Queue.
	taskQueue := queue.NewQueue(rdb, cfg.Worker.QueueName, cfg.Worker.DLQName)
	if err := taskQueue.Init(ctx); err != nil {
		logger.Warn(ctx, "queue init failed", zap.Error(err))
	}

	// MCP Registry.
	mcpRegistry := mcp.NewRegistry(mcpServerRepo)
	if err := mcpRegistry.LoadAll(ctx); err != nil {
		logger.Warn(ctx, "mcp registry load failed", zap.Error(err))
	}

	// Orchestrator.
	orchestrator := engine.NewOrchestrator(
		workflowRepo, executionRepo, taskRepo, taskQueue, metrics,
	)

	// Worker pool.
	workerID := "worker-" + uuid.NewString()[:8]
	pool := worker.NewPool(
		workerID,
		cfg.Worker,
		taskQueue,
		taskRepo,
		orchestrator,
		mcpRegistry,
		metrics,
	)

	logger.Info(ctx, "worker starting", zap.String("worker_id", workerID), zap.Int("concurrency", cfg.Worker.Concurrency))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go pool.Run(ctx)

	<-quit
	logger.Info(ctx, "worker shutting down...")
	cancel()
	// Give in-flight tasks a chance to complete.
	time.Sleep(cfg.Server.ShutdownTimeout)
	logger.Info(ctx, "worker shutdown complete")
}
