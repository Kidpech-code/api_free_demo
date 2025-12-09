package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/kidpech/api_free_demo/internal/domain/profile"
)

// ProfileRepository persists profiles via sqlx.
type ProfileRepository struct {
	db *sqlx.DB
}

// NewProfileRepository builds repo.
func NewProfileRepository(db *sqlx.DB) profile.Repository {
	return &ProfileRepository{db: db}
}

func (r *ProfileRepository) Create(ctx context.Context, p *profile.Profile) error {
	query := `INSERT INTO profiles (id, user_id, first_name, last_name, bio, profile_image, cover_image, date_of_birth,
		phone, website, location, created_at, updated_at, version)
		VALUES (:id, :user_id, :first_name, :last_name, :bio, :profile_image, :cover_image, :date_of_birth,
			:phone, :website, :location, :created_at, :updated_at, :version)`
	_, err := r.db.NamedExecContext(ctx, query, p)
	return err
}

func (r *ProfileRepository) BulkCreate(ctx context.Context, profiles []*profile.Profile) error {
	if len(profiles) == 0 {
		return nil
	}
	query := `INSERT INTO profiles (id, user_id, first_name, last_name, bio, profile_image, cover_image, date_of_birth,
		phone, website, location, created_at, updated_at, version)
		VALUES (:id, :user_id, :first_name, :last_name, :bio, :profile_image, :cover_image, :date_of_birth,
			:phone, :website, :location, :created_at, :updated_at, :version)`
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	for _, p := range profiles {
		if _, err := tx.NamedExecContext(ctx, query, p); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (r *ProfileRepository) Update(ctx context.Context, p *profile.Profile) error {
	query := `UPDATE profiles SET first_name = :first_name, last_name = :last_name, bio = :bio, profile_image = :profile_image,
		cover_image = :cover_image, date_of_birth = :date_of_birth, phone = :phone, website = :website, location = :location,
		updated_at = :updated_at, version = :version WHERE id = :id AND user_id = :user_id`
	res, err := r.db.NamedExecContext(ctx, query, p)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return profile.ErrNotFound
	}
	return nil
}

func (r *ProfileRepository) Patch(ctx context.Context, profileID uuid.UUID, userID uuid.UUID, fields map[string]interface{}, version int) (*profile.Profile, error) {
	setParts := make([]string, 0, len(fields)+2)
	args := make([]interface{}, 0, len(fields)+3)
	for key, val := range fields {
		setParts = append(setParts, fmt.Sprintf("%s = ?", key))
		args = append(args, val)
	}
	setParts = append(setParts, "updated_at = ?", "version = version + 1")
	args = append(args, time.Now().UTC())

	query := fmt.Sprintf("UPDATE profiles SET %s WHERE id = ? AND user_id = ? AND version = ?", strings.Join(setParts, ", "))
	args = append(args, profileID, userID, version)

	rebind := r.db.Rebind(query)
	res, err := r.db.ExecContext(ctx, rebind, args...)
	if err != nil {
		return nil, err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return nil, profile.ErrVersionConflict
	}
	return r.fetchByID(ctx, profileID)
}

func (r *ProfileRepository) Delete(ctx context.Context, profileID uuid.UUID, userID uuid.UUID, hard bool, version int) error {
	if hard {
		query := r.db.Rebind(`DELETE FROM profiles WHERE id = ? AND user_id = ? AND version = ?`)
		res, err := r.db.ExecContext(ctx, query, profileID, userID, version)
		if err != nil {
			return err
		}
		if count, _ := res.RowsAffected(); count == 0 {
			return profile.ErrNotFound
		}
		return nil
	}
	query := r.db.Rebind(`UPDATE profiles SET deleted_at = ?, version = version + 1 WHERE id = ? AND user_id = ? AND version = ?`)
	res, err := r.db.ExecContext(ctx, query, time.Now().UTC(), profileID, userID, version)
	if err != nil {
		return err
	}
	if count, _ := res.RowsAffected(); count == 0 {
		return profile.ErrVersionConflict
	}
	return nil
}

func (r *ProfileRepository) BulkDelete(ctx context.Context, userID uuid.UUID, ids []uuid.UUID, hard bool) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	query := `UPDATE profiles SET deleted_at = ? WHERE user_id = ? AND id IN (?)`
	if hard {
		query = `DELETE FROM profiles WHERE user_id = ? AND id IN (?)`
	}
	var args []interface{}
	if hard {
		query, args, _ = sqlx.In(query, userID, ids)
	} else {
		query, args, _ = sqlx.In(query, time.Now().UTC(), userID, ids)
	}
	rebind := r.db.Rebind(query)
	res, err := r.db.ExecContext(ctx, rebind, args...)
	if err != nil {
		return 0, err
	}
	count, _ := res.RowsAffected()
	return int(count), nil
}

func (r *ProfileRepository) GetByID(ctx context.Context, profileID uuid.UUID, userID uuid.UUID) (*profile.Profile, error) {
	var p profile.Profile
	query := r.db.Rebind(`SELECT * FROM profiles WHERE id = ? AND user_id = ? AND (deleted_at IS NULL)`)
	err := r.db.GetContext(ctx, &p, query, profileID, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, profile.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *ProfileRepository) List(ctx context.Context, filter profile.Filter) ([]profile.Profile, int, error) {
	base := `FROM profiles WHERE user_id = ? AND (deleted_at IS NULL)`
	args := []interface{}{filter.UserID}
	if filter.Search != "" {
		args = append(args, "%"+filter.Search+"%", "%"+filter.Search+"%")
		base += ` AND (LOWER(first_name) LIKE LOWER(?) OR LOWER(last_name) LIKE LOWER(?))`
	}
	if filter.CursorTime != nil {
		args = append(args, *filter.CursorTime)
		base += ` AND created_at < ?`
	}
	countArgs := append([]interface{}{}, args...)
	query := r.db.Rebind("SELECT * " + base + " ORDER BY created_at DESC LIMIT ? OFFSET ?")
	queryArgs := append(countArgs, filter.Limit, filter.Offset)
	var profilesList []profile.Profile
	if err := r.db.SelectContext(ctx, &profilesList, query, queryArgs...); err != nil {
		return nil, 0, err
	}
	countQuery := r.db.Rebind("SELECT COUNT(*) " + base)
	var total int
	if err := r.db.GetContext(ctx, &total, countQuery, countArgs...); err != nil {
		return nil, 0, err
	}
	return profilesList, total, nil
}

func (r *ProfileRepository) fetchByID(ctx context.Context, id uuid.UUID) (*profile.Profile, error) {
	var p profile.Profile
	query := r.db.Rebind(`SELECT * FROM profiles WHERE id = ?`)
	err := r.db.GetContext(ctx, &p, query, id)
	if err != nil {
		return nil, err
	}
	return &p, nil
}
