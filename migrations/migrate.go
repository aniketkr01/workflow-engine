package migrations

import (
	"context"
	"embed"
	"io/fs"
	"sort"
	"strings"

	"github.com/aniketkr01/workflow-engine/internal/logger"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

//go:embed *.sql
var sqlFiles embed.FS

// Run executes all migration files in the embedded filesystem 
// against the provided database connection pool.
func Run(ctx context.Context, pool *pgxpool.Pool) error {
	entries, err := fs.ReadDir(sqlFiles, ".")
	if err != nil {
		logger.Error(ctx, "failed to read migration files", zap.Error(err))
		return err
	}

	var files []string

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}

	sort.Strings(files)

	for _, file := range files {
		raw, err := sqlFiles.ReadFile(file)
		if err != nil {
			logger.Error(ctx, "failed to read migration file", zap.String("file", file), zap.Error(err))
			return err
		}
		sql := extractUpSection(string(raw))
		if _, err := pool.Exec(ctx, sql); err != nil {
			logger.Error(ctx, "failed to execute migration", zap.String("file", file), zap.Error(err))
			return err
		}
		logger.Info(ctx, "successfully applied migration", zap.String("file", file))
	}
	return nil
}

// extractUpSection extracts the SQL statements from the "Up" 
// section of a migration file, ignoring any "Down" section.
func extractUpSection(content string) string {
	const upMarker = "-- +migrate Up"
	const downMarker = "-- +migrate Down"

	start := strings.Index(content, upMarker)
	if start == -1 {
		return content
	}
	content = content[start+len(upMarker):]
	if end := strings.Index(content, downMarker); end != -1 {
		content = content[:end]
	}
	return strings.TrimSpace(content)
}
