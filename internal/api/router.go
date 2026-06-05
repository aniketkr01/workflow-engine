package api

import (
	"net/http"

	"github.com/aniketkr01/workflow-engine/internal/api/handlers"
	apimiddleware "github.com/aniketkr01/workflow-engine/internal/api/middleware"
	"github.com/aniketkr01/workflow-engine/internal/auth"
	"github.com/aniketkr01/workflow-engine/internal/domain"
	"github.com/aniketkr01/workflow-engine/internal/telemetry"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	otelgin "go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

// NewRouter builds and returns the Gin router.
func NewRouter(
	authHandler *handlers.AuthHandler,
	workflowHandler *handlers.WorkflowHandler,
	mcpHandler *handlers.MCPHandler,
	jwtManager *auth.JWTManager,
	metrics *telemetry.Metrics,
) *gin.Engine {
	r := gin.New()

	// Global middleware.
	r.Use(apimiddleware.Recovery())
	r.Use(otelgin.Middleware("workflow-engine"))
	r.Use(apimiddleware.RequestLogger())
	r.Use(apimiddleware.PrometheusMiddleware(metrics))

	// Health & metrics.
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	api := r.Group("/api/v1")

	// Auth routes (public).
	authGroup := api.Group("/auth")
	{
		authGroup.POST("/register", authHandler.Register)
		authGroup.POST("/login", authHandler.Login)
	}

	// Authenticated routes.
	protected := api.Group("/")
	protected.Use(auth.Authenticate(jwtManager))
	{
		// Me
		protected.GET("auth/me", authHandler.Me)

		// Workflows.
		protected.POST("workflows", workflowHandler.CreateWorkflow)
		protected.GET("workflows", workflowHandler.ListWorkflows)
		protected.GET("workflows/:id", workflowHandler.GetWorkflow)
		protected.PUT("workflows/:id", workflowHandler.UpdateWorkflow)
		protected.DELETE("workflows/:id", auth.RequireRole(domain.RoleAdmin, domain.RoleOperator), workflowHandler.DeleteWorkflow)
		protected.POST("workflows/:id/trigger", workflowHandler.TriggerWorkflow)
		protected.GET("workflows/:id/executions", workflowHandler.ListExecutions)

		// Executions.
		protected.GET("executions/:id", workflowHandler.GetExecution)
		protected.POST("executions/:id/cancel", workflowHandler.CancelExecution)
		protected.GET("executions/:id/tasks", workflowHandler.GetExecutionTasks)

		// MCP servers (admin only for management).
		mcpGroup := protected.Group("mcp")
		mcpGroup.Use(auth.RequireRole(domain.RoleAdmin))
		{
			mcpGroup.POST("servers", mcpHandler.RegisterServer)
			mcpGroup.GET("servers", mcpHandler.ListServers)
			mcpGroup.DELETE("servers/:id", mcpHandler.DeleteServer)
		}
		protected.GET("mcp/tools", mcpHandler.ListTools) // all authenticated users
	}

	return r
}
