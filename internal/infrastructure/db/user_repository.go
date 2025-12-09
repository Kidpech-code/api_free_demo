package db

import (
	"context"
	"database/sql"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/kidpech/api_free_demo/internal/domain/user"
)

// UserRepository implements user.Repository using sqlx.
type UserRepository struct {
	db *sqlx.DB
}

// NewUserRepository constructs the repo.
func NewUserRepository(db *sqlx.DB) user.Repository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, u *user.User) error {
	query := `INSERT INTO users (id, email, name, password_hash, profile_image, role, refresh_version, created_at, updated_at)
		VALUES (:id, :email, :name, :password_hash, :profile_image, :role, :refresh_version, :created_at, :updated_at)`
	_, err := r.db.NamedExecContext(ctx, query, u)
	if err != nil {
		if isDuplicate(err) {
			return user.ErrDuplicateEmail
		}
	}
	return err
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
	err := r.db.GetContext(ctx, &u, query, email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	var u user.User
	query := r.db.Rebind(`SELECT * FROM users WHERE id = ? AND deleted_at IS NULL`)
	err := r.db.GetContext(ctx, &u, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, user.ErrUserNotFound
		}
		return nil, err
	}
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
