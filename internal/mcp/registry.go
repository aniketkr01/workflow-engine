package mcp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aniketkr01/workflow-engine/internal/domain"
	"github.com/aniketkr01/workflow-engine/internal/logger"
	"github.com/aniketkr01/workflow-engine/internal/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Registry manages live MCP client connections and their discovered tools.
type Registry struct {
	repo    repository.MCPServerRepository
	clients sync.Map // name -> Client
	tools   sync.Map // name -> []domain.MCPTool
}

func NewRegistry(repo repository.MCPServerRepository) *Registry {
	return &Registry{repo: repo}
}

// LoadAll connects to all enabled MCP servers and discovers their tools.
func (r *Registry) LoadAll(ctx context.Context) error {
	servers, err := r.repo.List(ctx)
	if err != nil {
		return fmt.Errorf("list mcp servers: %w", err)
	}
	for _, srv := range servers {
		if !srv.Enabled {
			continue
		}
		if err := r.Connect(ctx, srv); err != nil {
			logger.Error(ctx, "failed to connect mcp server", zap.String("server", srv.Name), zap.Error(err))
		}
	}
	return nil
}

// Connect creates a client for the server and discovers tools.
func (r *Registry) Connect(ctx context.Context, srv *domain.MCPServer) error {
	client, err := NewClient(srv)
	if err != nil {
		return fmt.Errorf("create mcp client: %w", err)
	}

	ctx2, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if _, err := client.Initialize(ctx2); err != nil {
		_ = client.Close()
		return fmt.Errorf("initialize mcp server %s: %w", srv.Name, err)
	}

	tools, err := client.ListTools(ctx2)
	if err != nil {
		_ = client.Close()
		return fmt.Errorf("list tools from %s: %w", srv.Name, err)
	}

	mcpTools := make([]domain.MCPTool, len(tools))
	for i, t := range tools {
		mcpTools[i] = domain.MCPTool{
			ServerID:    srv.ID,
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		}
	}

	r.clients.Store(srv.Name, client)
	r.tools.Store(srv.Name, mcpTools)

	logger.Info(ctx, "mcp server connected", zap.String("server", srv.Name), zap.Int("tools", len(tools)))
	return nil
}

// Disconnect removes a client from the registry.
func (r *Registry) Disconnect(name string) {
	if v, ok := r.clients.LoadAndDelete(name); ok {
		_ = v.(Client).Close()
	}
	r.tools.Delete(name)
}

// GetClient returns the live client for a server by name.
func (r *Registry) GetClient(name string) (Client, bool) {
	v, ok := r.clients.Load(name)
	if !ok {
		return nil, false
	}
	return v.(Client), true
}

// ListAllTools returns all discovered tools across all connected servers.
func (r *Registry) ListAllTools() []domain.MCPTool {
	var all []domain.MCPTool
	r.tools.Range(func(_, v any) bool {
		all = append(all, v.([]domain.MCPTool)...)
		return true
	})
	return all
}

// GetTool resolves which server+client handles a given tool name.
func (r *Registry) GetTool(serverName, toolName string) (Client, *domain.MCPTool, error) {
	client, ok := r.GetClient(serverName)
	if !ok {
		return nil, nil, fmt.Errorf("mcp server %q not connected", serverName)
	}
	v, ok := r.tools.Load(serverName)
	if !ok {
		return nil, nil, fmt.Errorf("no tools loaded for server %q", serverName)
	}
	for _, t := range v.([]domain.MCPTool) {
		if t.Name == toolName {
			return client, &t, nil
		}
	}
	return nil, nil, fmt.Errorf("tool %q not found on server %q", toolName, serverName)
}

// RegisterServer persists and connects a new MCP server.
func (r *Registry) RegisterServer(ctx context.Context, srv *domain.MCPServer) error {
	if srv.ID == uuid.Nil {
		srv.ID = uuid.New()
	}
	if err := r.repo.Create(ctx, srv); err != nil {
		return fmt.Errorf("persist mcp server: %w", err)
	}
	return r.Connect(ctx, srv)
}
