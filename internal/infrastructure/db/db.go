package db

import (
	"context"
	"fmt"
	"strings"
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
	maxRetries := 10
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
			if waitTime > 10*time.Second {
				waitTime = 10 * time.Second
			}
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

	// Auto-migrate: create tables if they don't exist (idempotent)
	if cfg.AutoMigrate && strings.Contains(strings.ToLower(cfg.Driver), "postgres") {
		if err := autoMigratePostgres(write, logger); err != nil {
			if logger != nil {
				logger.Warn("auto-migration failed (tables may already exist)", zap.Error(err))
			}
		}
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

// autoMigratePostgres runs idempotent schema creation for Postgres.
func autoMigratePostgres(db *sqlx.DB, logger *zap.Logger) error {
	migration := `
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY,
    email CITEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    profile_image TEXT,
    role TEXT NOT NULL DEFAULT 'user',
    refresh_version INT NOT NULL DEFAULT 1,
    last_login_at TIMESTAMPTZ,
    password_reset_at TIMESTAMPTZ,
    last_password_hash TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS profiles (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    first_name TEXT NOT NULL,
    last_name TEXT NOT NULL,
    bio TEXT,
    profile_image TEXT,
    cover_image TEXT,
    date_of_birth DATE,
    phone TEXT,
    website TEXT,
    location TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    version INT NOT NULL DEFAULT 1
);

CREATE INDEX IF NOT EXISTS idx_profiles_user_id ON profiles(user_id);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(LOWER(email));
`

	seed := `
INSERT INTO users (id, email, name, password_hash, role)
VALUES
    ('00000000-0000-0000-0000-000000000001', 'admin@kidpech.app', 'Demo Admin',
     '$2a$12$LbhpmsYNIQP5CKEM5Qn2jOBYx8RJWb1x1My1t4bm/F/6HC8K3oprm', 'admin')
ON CONFLICT (email) DO NOTHING;

INSERT INTO profiles (id, user_id, first_name, last_name, bio, created_at, updated_at)
VALUES
    ('10000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000001',
     'Demo', 'Admin', 'System seeded profile', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;
`

	if _, err := db.Exec(migration); err != nil {
		return fmt.Errorf("migration: %w", err)
	}
	if logger != nil {
		logger.Info("auto-migration completed successfully")
	}

	if _, err := db.Exec(seed); err != nil {
		if logger != nil {
			logger.Warn("seed data insert skipped or failed", zap.Error(err))
		}
		// Not fatal — tables exist, seed may have run before
	} else if logger != nil {
		logger.Info("seed data applied")
	}

	return nil
}
