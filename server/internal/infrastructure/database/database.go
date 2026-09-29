// Package database 封装 MySQL（GORM）连接。
package database

import (
	"fmt"
	"time"

	"citywall/server/internal/config"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Open 建立 MySQL 连接并返回 *gorm.DB。
// dev 阶段 MySQL 未就绪且 config.Database.Required=false 时返回 nil（调用方降级启动）。
func Open(cfg config.DatabaseConfig) (*gorm.DB, error) {
	level, err := parseLogLevel(cfg.LogLevel)
	if err != nil {
		return nil, err
	}

	db, err := gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{
		Logger: gormlogger.Default.LogMode(level),
	})
	if err != nil {
		return nil, fmt.Errorf("connect mysql: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql db: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetimeMin) * time.Minute)

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("ping mysql: %w", err)
	}
	return db, nil
}

func parseLogLevel(s string) (gormlogger.LogLevel, error) {
	switch s {
	case "silent":
		return gormlogger.Silent, nil
	case "error":
		return gormlogger.Error, nil
	case "warn":
		return gormlogger.Warn, nil
	case "info":
		return gormlogger.Info, nil
	case "":
		return gormlogger.Warn, nil
	default:
		return gormlogger.Silent, fmt.Errorf("database: unknown log_level %q", s)
	}
}
