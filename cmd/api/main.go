package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/aniketkr01/workflow-engine/internal/api"
	"github.com/aniketkr01/workflow-engine/internal/api/handlers"
	"github.com/aniketkr01/workflow-engine/internal/auth"
	"github.com/aniketkr01/workflow-engine/internal/config"
	"github.com/aniketkr01/workflow-engine/internal/engine"
	"github.com/aniketkr01/workflow-engine/internal/grpcserver"
	"github.com/aniketkr01/workflow-engine/internal/logger"
	"github.com/aniketkr01/workflow-engine/internal/mcp"
	"github.com/aniketkr01/workflow-engine/internal/queue"
	"github.com/aniketkr01/workflow-engine/internal/repository/postgres"
	"github.com/aniketkr01/workflow-engine/internal/telemetry"
	"github.com/aniketkr01/workflow-engine/migrations"
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
		shutdown, err := telemetry.InitTracing(ctx, cfg.Telemetry.ServiceName, cfg.Telemetry.OTELEndpoint)
		if err != nil {
			logger.Warn(ctx, "tracing init failed", zap.Error(err))
		} else {
			defer func() {
				shutdownCtx, c := context.WithTimeout(context.Background(), 5*time.Second)
				defer c()
				if err = shutdown(shutdownCtx); err != nil {
					logger.Error(ctx, "tracing shutdown failed", zap.Error(err))
				}
			}()
		}
	}

	// Database.
	db, err := postgres.NewPool(ctx, cfg.Database)
	if err != nil {
		logger.Fatal(ctx, "connect to database", zap.Error(err))
	}
	defer db.Close()

	// Database Migrations.
	if err := migrations.Run(ctx, db); err != nil {
		logger.Fatal(ctx, "database migrations failed", zap.Error(err))
	}

	// Redis.
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer rdb.Close()

	// Repositories.
	userRepo := postgres.NewUserRepo(db)
	workflowRepo := postgres.NewWorkflowRepo(db)
	executionRepo := postgres.NewExecutionRepo(db)
	taskRepo := postgres.NewTaskRepo(db)
	mcpServerRepo := postgres.NewMCPServerRepo(db)

	// Metrics.
	metrics := telemetry.NewMetrics()

	// Queue.
	taskQueue := queue.NewQueue(rdb, cfg.Worker.QueueName, cfg.Worker.DLQName)
	if err := taskQueue.Init(ctx); err != nil {
		logger.Fatal(ctx, "queue init failed", zap.Error(err))
	}

	// MCP Registry.
	mcpRegistry := mcp.NewRegistry(mcpServerRepo)
	if err := mcpRegistry.LoadAll(ctx); err != nil {
		logger.Fatal(ctx, "mcp registry load failed", zap.Error(err))
	}

	// Orchestrator.
	orchestrator := engine.NewOrchestrator(
		workflowRepo, executionRepo, taskRepo, taskQueue, metrics,
	)

	// Scheduler.
	scheduler := engine.NewScheduler(
		workflowRepo, executionRepo, orchestrator, 30*time.Second,
	)
	go scheduler.Run(ctx)

	// Auth.
	jwtManager := auth.NewJWTManager(cfg.Auth.JWTSecret, cfg.Auth.TokenDuration)

	// HTTP Handlers.
	authHandler := handlers.NewAuthHandler(userRepo, jwtManager)
	workflowHandler := handlers.NewWorkflowHandler(workflowRepo, executionRepo, taskRepo, orchestrator)
	mcpHandler := handlers.NewMCPHandler(mcpRegistry)

	// HTTP Router.
	router := api.NewRouter(authHandler, workflowHandler, mcpHandler, jwtManager, metrics)

	httpServer := &http.Server{
		Addr:         "0.0.0.0:" + cfg.Server.HTTPPort,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// gRPC Server.
	grpcSrv := grpcserver.NewServer(orchestrator, executionRepo, taskRepo)
	grpcListener, err := net.Listen("tcp", ":"+cfg.Server.GRPCPort)
	if err != nil {
		logger.Fatal(ctx, "grpc listen", zap.Error(err))
	}

	// Start HTTP.
	go func() {
		logger.Info(ctx, "HTTP server starting",
			zap.String("port", cfg.Server.HTTPPort))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal(ctx, "HTTP server failed", zap.Error(err))
		}
	}()

	// Start gRPC.
	go func() {
		logger.Info(ctx, "gRPC server starting",
			zap.String("port", cfg.Server.GRPCPort))
		if err := grpcSrv.GRPCServer().Serve(grpcListener); err != nil && err != grpc.ErrServerStopped {
			logger.Error(ctx, "gRPC server failed", zap.Error(err))
		}
	}()

	// Queue depth reporter.
	go reportQueueDepth(ctx, taskQueue, metrics)

	// Graceful shutdown.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info(ctx, "shutting down...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error(ctx, "HTTP server shutdown error", zap.Error(err))
	}
	grpcSrv.GRPCServer().GracefulStop()
	logger.Info(ctx, "shutdown complete")
}

func reportQueueDepth(ctx context.Context, q *queue.Queue, metrics *telemetry.Metrics) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			depth, err := q.Len(ctx)
			if err == nil {
				metrics.QueueDepth.Set(float64(depth))
			}
		}
	}
}
