package db

import (
	"context"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/kidpech/api_free_demo/internal/config"
)

// Manager coordinates read/write connections.
type Manager struct {
	Write *sqlx.DB
	Read  *sqlx.DB
}

// Connect establishes sqlx connections based on configuration.
func Connect(ctx context.Context, cfg config.DatabaseConfig, logger *zap.Logger) (*Manager, error) {
	// sqlx driver name mapping: allow "postgres" in config but use the
	// compiled pgx stdlib driver which registers under "pgx".
	driverName := cfg.Driver
	if driverName == "postgres" {
		driverName = "pgx"
	}

	write, err := sqlx.Open(driverName, cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	write.SetMaxOpenConns(cfg.MaxOpenConns)
	write.SetMaxIdleConns(cfg.MaxIdleConns)
	write.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	// Retry connection with exponential backoff for Railway startup
	var pingErr error
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		pingErr = write.PingContext(ctx)
		cancel()
		
		if pingErr == nil {
			break
		}
		
		if logger != nil {
			logger.Warn("db ping failed, retrying...", 
				zap.Int("attempt", i+1), 
				zap.Int("max_retries", maxRetries),
				zap.Error(pingErr))
		}
		
		if i < maxRetries-1 {
			waitTime := time.Duration(i+1) * 2 * time.Second
			time.Sleep(waitTime)
		}
	}
	
	if pingErr != nil {
		write.Close()
		return nil, fmt.Errorf("ping db after %d retries: %w", maxRetries, pingErr)
	}

	if logger != nil {
		logger.Info("database connected successfully")
	}

	mgr := &Manager{Write: write, Read: write}
	if cfg.ReadOnlyDSN != "" {
		read, err := sqlx.Open(driverName, cfg.ReadOnlyDSN)
		if err != nil {
			if logger != nil {
				logger.Warn("read-only db open failed", zap.Error(err))
			}
		} else {
			read.SetMaxOpenConns(cfg.MaxOpenConns)
			read.SetMaxIdleConns(cfg.MaxIdleConns)
			read.SetConnMaxLifetime(cfg.ConnMaxLifetime)
			if err := read.PingContext(ctx); err != nil {
				if logger != nil {
					logger.Warn("read-only db ping failed", zap.Error(err))
				}
				_ = read.Close()
			} else {
				mgr.Read = read
			}
		}
	}

	return mgr, nil
}

// Close closes all DB handles.
func (m *Manager) Close() error {
	if m == nil || m.Write == nil {
		return nil
	}
	if err := m.Write.Close(); err != nil {
		return err
	}
	if m.Read != nil && m.Read != m.Write {
		return m.Read.Close()
	}
	return nil
}
