package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/aniketkr01/workflow-engine/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MCPServerRepo struct {
	db *pgxpool.Pool
}

func NewMCPServerRepo(db *pgxpool.Pool) *MCPServerRepo {
	return &MCPServerRepo{db: db}
}

func (r *MCPServerRepo) Create(ctx context.Context, srv *domain.MCPServer) error {
	if srv.ID == uuid.Nil {
		srv.ID = uuid.New()
	}
	now := time.Now()
	srv.CreatedAt = now
	srv.UpdatedAt = now

	_, err := r.db.Exec(ctx,
		`INSERT INTO mcp_servers (id, name, transport, endpoint, enabled, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		srv.ID, srv.Name, srv.Transport, srv.Endpoint, srv.Enabled, srv.CreatedAt, srv.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create mcp server: %w", err)
	}
	return nil
}

func (r *MCPServerRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.MCPServer, error) {
	var srv domain.MCPServer
	err := r.db.QueryRow(ctx,
		`SELECT id, name, transport, endpoint, enabled, created_at, updated_at FROM mcp_servers WHERE id = $1`, id,
	).Scan(&srv.ID, &srv.Name, &srv.Transport, &srv.Endpoint, &srv.Enabled, &srv.CreatedAt, &srv.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get mcp server: %w", err)
	}
	return &srv, nil
}

func (r *MCPServerRepo) GetByName(ctx context.Context, name string) (*domain.MCPServer, error) {
	var srv domain.MCPServer
	err := r.db.QueryRow(ctx,
		`SELECT id, name, transport, endpoint, enabled, created_at, updated_at FROM mcp_servers WHERE name = $1`, name,
	).Scan(&srv.ID, &srv.Name, &srv.Transport, &srv.Endpoint, &srv.Enabled, &srv.CreatedAt, &srv.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get mcp server by name: %w", err)
	}
	return &srv, nil
}

func (r *MCPServerRepo) List(ctx context.Context) ([]*domain.MCPServer, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, name, transport, endpoint, enabled, created_at, updated_at FROM mcp_servers ORDER BY name ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list mcp servers: %w", err)
	}
	defer rows.Close()

	var servers []*domain.MCPServer
	for rows.Next() {
		var srv domain.MCPServer
		if err := rows.Scan(&srv.ID, &srv.Name, &srv.Transport, &srv.Endpoint,
			&srv.Enabled, &srv.CreatedAt, &srv.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan mcp server: %w", err)
		}
		servers = append(servers, &srv)
	}
	return servers, nil
}

func (r *MCPServerRepo) Update(ctx context.Context, srv *domain.MCPServer) error {
	srv.UpdatedAt = time.Now()
	_, err := r.db.Exec(ctx,
		`UPDATE mcp_servers SET name=$1, transport=$2, endpoint=$3, enabled=$4, updated_at=$5 WHERE id=$6`,
		srv.Name, srv.Transport, srv.Endpoint, srv.Enabled, srv.UpdatedAt, srv.ID,
	)
	if err != nil {
		return fmt.Errorf("update mcp server: %w", err)
	}
	return nil
}

func (r *MCPServerRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM mcp_servers WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete mcp server: %w", err)
	}
	return nil
}
