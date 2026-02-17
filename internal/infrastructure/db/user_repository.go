package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/kidpech/api_free_demo/internal/domain/user"
)

// UserRepository implements user.Repository using sqlx.
type UserRepository struct {
	db     *sqlx.DB
	logger *zap.Logger
}

// NewUserRepository constructs the repo.
func NewUserRepository(db *sqlx.DB) user.Repository {
	return &UserRepository{db: db, logger: zap.L()}
}

func (r *UserRepository) Create(ctx context.Context, u *user.User) error {
	query := `INSERT INTO users (id, email, name, password_hash, profile_image, role, refresh_version, created_at, updated_at)
		VALUES (:id, :email, :name, :password_hash, :profile_image, :role, :refresh_version, :created_at, :updated_at)`

	r.logger.Info("[DB] Attempting user.Create",
		zap.String("id", u.ID.String()),
		zap.String("email", u.Email),
	)

	result, err := r.db.NamedExecContext(ctx, query, u)
	if err != nil {
		r.logger.Error("[DB] user.Create exec failed",
			zap.String("id", u.ID.String()),
			zap.Error(err),
		)
		if isDuplicate(err) {
			return user.ErrDuplicateEmail
		}
		return fmt.Errorf("user create exec failed: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		r.logger.Error("[DB] user.Create rows check failed",
			zap.String("id", u.ID.String()),
			zap.Error(err),
		)
		return fmt.Errorf("user create rows check failed: %w", err)
	}

	if rows == 0 {
		r.logger.Error("[DB] user.Create: zero rows affected!",
			zap.String("id", u.ID.String()),
			zap.String("email", u.Email),
		)
		return fmt.Errorf("user create: no rows inserted (id=%s, email=%s)", u.ID, u.Email)
	}

	r.logger.Info("[DB] user.Create SUCCESS",
		zap.String("id", u.ID.String()),
		zap.String("email", u.Email),
		zap.Int64("rows", rows),
	)
	return nil
}

func (r *UserRepository) Update(ctx context.Context, u *user.User) error {
	query := `UPDATE users SET name = :name, profile_image = :profile_image, password_hash = :password_hash,
		refresh_version = :refresh_version, updated_at = :updated_at, last_login_at = :last_login_at WHERE id = :id`
	_, err := r.db.NamedExecContext(ctx, query, u)
	return err
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	var u user.User
	query := r.db.Rebind(`SELECT * FROM users WHERE LOWER(email) = LOWER(?) AND deleted_at IS NULL LIMIT 1`)

	r.logger.Debug("[DB] GetByEmail query",
		zap.String("email", email),
		zap.String("query", query),
	)

	err := r.db.GetContext(ctx, &u, query, email)
	if err != nil {
		if err == sql.ErrNoRows {
			r.logger.Debug("[DB] GetByEmail: not found", zap.String("email", email))
			return nil, nil
		}
		r.logger.Error("[DB] GetByEmail failed",
			zap.String("email", email),
			zap.Error(err),
		)
		return nil, err
	}

	r.logger.Info("[DB] GetByEmail SUCCESS",
		zap.String("email", email),
		zap.String("found_id", u.ID.String()),
	)
	return &u, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	var u user.User
	query := r.db.Rebind(`SELECT * FROM users WHERE id = ? AND deleted_at IS NULL`)

	r.logger.Debug("[DB] GetByID query",
		zap.String("id", id.String()),
		zap.String("query", query),
	)

	err := r.db.GetContext(ctx, &u, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			r.logger.Warn("[DB] GetByID: not found (should exist after Create!)",
				zap.String("id", id.String()),
			)
			return nil, user.ErrUserNotFound
		}
		r.logger.Error("[DB] GetByID failed",
			zap.String("id", id.String()),
			zap.Error(err),
		)
		return nil, err
	}

	r.logger.Info("[DB] GetByID SUCCESS",
		zap.String("id", id.String()),
		zap.String("email", u.Email),
	)
	return &u, nil
}

func (r *UserRepository) List(ctx context.Context, filter user.UserFilter) ([]user.User, int, error) {
	where := []string{"deleted_at IS NULL"}
	params := []interface{}{}
	if filter.Search != "" {
		where = append(where, "(LOWER(email) LIKE LOWER(?) OR LOWER(name) LIKE LOWER(?))")
		params = append(params, "%"+filter.Search+"%", "%"+filter.Search+"%")
	}
	base := "FROM users WHERE " + strings.Join(where, " AND ")
	order := "created_at DESC"
	if strings.Contains(filter.Sort, "name") {
		order = "name ASC"
	}
	query := r.db.Rebind("SELECT * " + base + " ORDER BY " + order + " LIMIT ? OFFSET ?")
	var users []user.User
	queryArgs := append(append([]interface{}{}, params...), filter.Limit, filter.Offset)
	if err := r.db.SelectContext(ctx, &users, query, queryArgs...); err != nil {
		return nil, 0, err
	}
	countQuery := r.db.Rebind("SELECT COUNT(*) " + base)
	var total int
	if err := r.db.GetContext(ctx, &total, countQuery, params...); err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

func isDuplicate(err error) bool {
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "duplicate") || strings.Contains(s, "unique")
}
