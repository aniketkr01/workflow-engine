package handlers

import (
	"net/http"
	"strconv"

	"github.com/aniketkr01/workflow-engine/internal/auth"
	"github.com/aniketkr01/workflow-engine/internal/domain"
	"github.com/aniketkr01/workflow-engine/internal/engine"
	"github.com/aniketkr01/workflow-engine/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type WorkflowHandler struct {
	workflowRepo  repository.WorkflowRepository
	executionRepo repository.ExecutionRepository
	taskRepo      repository.TaskRepository
	orchestrator  *engine.Orchestrator
}

func NewWorkflowHandler(
	wfRepo repository.WorkflowRepository,
	execRepo repository.ExecutionRepository,
	taskRepo repository.TaskRepository,
	orch *engine.Orchestrator,
) *WorkflowHandler {
	return &WorkflowHandler{
		workflowRepo:  wfRepo,
		executionRepo: execRepo,
		taskRepo:      taskRepo,
		orchestrator:  orch,
	}
}

// CreateWorkflow
// POST /workflows
func (h *WorkflowHandler) CreateWorkflow(c *gin.Context) {
	claims, _ := auth.GetClaims(c)
	ownerID, err := claims.GetUserID()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	var wf domain.Workflow
	if err := c.ShouldBindJSON(&wf); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	wf.OwnerID = ownerID
	wf.Status = domain.WorkflowStatusDraft

	if err := h.workflowRepo.Create(c.Request.Context(), &wf); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, wf)
}

// ListWorkflows
// GET /workflows
func (h *WorkflowHandler) ListWorkflows(c *gin.Context) {
	claims, _ := auth.GetClaims(c)
	ownerID, _ := claims.GetUserID()

	limit := 20
	offset := 0
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	workflows, total, err := h.workflowRepo.List(c.Request.Context(), ownerID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"workflows": workflows,
		"total":     total,
		"limit":     limit,
		"offset":    offset,
	})
}

// GetWorkflow
// GET /workflows/:id
func (h *WorkflowHandler) GetWorkflow(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid workflow id"})
		return
	}
	wf, err := h.workflowRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow not found"})
		return
	}
	c.JSON(http.StatusOK, wf)
}

// UpdateWorkflow
// PUT /workflows/:id
func (h *WorkflowHandler) UpdateWorkflow(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid workflow id"})
		return
	}

	existing, err := h.workflowRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow not found"})
		return
	}

	var update domain.Workflow
	if err := c.ShouldBindJSON(&update); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	existing.Name = update.Name
	existing.Description = update.Description
	existing.Tasks = update.Tasks
	existing.Schedule = update.Schedule
	existing.Version++

	if err := h.workflowRepo.Update(c.Request.Context(), existing); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, existing)
}

// DeleteWorkflow
// DELETE /workflows/:id
func (h *WorkflowHandler) DeleteWorkflow(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid workflow id"})
		return
	}
	if err := h.workflowRepo.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// TriggerWorkflow godoc
// POST /workflows/:id/trigger
func (h *WorkflowHandler) TriggerWorkflow(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid workflow id"})
		return
	}

	exec, err := h.orchestrator.StartExecution(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, exec)
}

// GetExecution godoc
// GET /executions/:id
func (h *WorkflowHandler) GetExecution(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid execution id"})
		return
	}
	exec, err := h.executionRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "execution not found"})
		return
	}
	c.JSON(http.StatusOK, exec)
}

// ListExecutions
// GET /workflows/:id/executions
func (h *WorkflowHandler) ListExecutions(c *gin.Context) {
	wfID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid workflow id"})
		return
	}

	limit := 20
	offset := 0
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			offset = n
		}
	}

	execs, err := h.executionRepo.ListByWorkflow(c.Request.Context(), wfID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"executions": execs})
}

// CancelExecution
// POST /executions/:id/cancel
func (h *WorkflowHandler) CancelExecution(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid execution id"})
		return
	}
	if err := h.orchestrator.CancelExecution(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "execution cancelled"})
}

// GetExecutionTasks
// GET /executions/:id/tasks
func (h *WorkflowHandler) GetExecutionTasks(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid execution id"})
		return
	}
	tasks, err := h.taskRepo.ListByExecution(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"tasks": tasks})
}
