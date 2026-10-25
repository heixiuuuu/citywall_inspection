package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Migrate 使用同一连接持有数据库锁，避免多个进程重复执行 DDL。
func Migrate(db *sql.DB, directory string) error {
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	var acquired int
	if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK(CONCAT(DATABASE(), ':schema'), 60)").Scan(&acquired); err != nil {
		return err
	}
	if acquired != 1 {
		return fmt.Errorf("database migration lock unavailable")
	}
	defer conn.ExecContext(ctx, "SELECT RELEASE_LOCK(CONCAT(DATABASE(), ':schema'))")
	if _, err := conn.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations (version VARCHAR(128) PRIMARY KEY, applied_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4"); err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(directory, "*.sql"))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no migrations found in %s", directory)
	}
	sort.Strings(files)
	for _, path := range files {
		version := strings.TrimSuffix(filepath.Base(path), ".sql")
		var applied int
		if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE version = ?", version).Scan(&applied); err != nil {
			return err
		}
		if applied > 0 {
			continue
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if _, err := conn.ExecContext(ctx, string(contents)); err != nil {
			return fmt.Errorf("migration %s: %w", version, err)
		}
		if _, err := conn.ExecContext(ctx, "INSERT INTO schema_migrations(version) VALUES (?)", version); err != nil {
			return err
		}
	}
	return nil
}
