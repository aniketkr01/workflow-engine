package handlers

import (
	"net/http"

	"github.com/aniketkr01/workflow-engine/internal/domain"
	"github.com/aniketkr01/workflow-engine/internal/mcp"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type MCPHandler struct {
	registry *mcp.Registry
}

func NewMCPHandler(registry *mcp.Registry) *MCPHandler {
	return &MCPHandler{registry: registry}
}

// RegisterServer
// POST /mcp/servers
func (h *MCPHandler) RegisterServer(c *gin.Context) {
	var srv domain.MCPServer
	if err := c.ShouldBindJSON(&srv); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.registry.RegisterServer(c.Request.Context(), &srv); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, srv)
}

// ListServers
// GET /mcp/servers
func (h *MCPHandler) ListServers(c *gin.Context) {
	// List all tools gives us an indirect view; for server list we use the repo via registry.
	tools := h.registry.ListAllTools()
	c.JSON(http.StatusOK, gin.H{"tools": tools})
}

// ListTools
// GET /mcp/tools
func (h *MCPHandler) ListTools(c *gin.Context) {
	tools := h.registry.ListAllTools()
	c.JSON(http.StatusOK, gin.H{"tools": tools, "count": len(tools)})
}

// DeleteServer
// DELETE /mcp/servers/:id
func (h *MCPHandler) DeleteServer(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid server id"})
		return
	}
	_ = id
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not yet implemented"})
}
