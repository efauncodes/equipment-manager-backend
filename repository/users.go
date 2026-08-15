package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/efauncodes/equipment-manager-backend/domain"
)

type UserRepository struct{ store *Store }

func (s *Store) Users() *UserRepository { return &UserRepository{store: s} }

func (r *UserRepository) Create(ctx context.Context, user domain.User) error {
	_, err := r.store.DB.ExecContext(ctx, `INSERT INTO users(id,email,display_name,role,is_active,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, user.ID, user.Email, user.DisplayName, user.Role, boolInt(user.IsActive), user.CreatedAt, user.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (r *UserRepository) Get(ctx context.Context, id string) (domain.User, error) {
	return scanUser(r.store.DB.QueryRowContext(ctx, userQuery+` WHERE id = ?`, id))
}
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (domain.User, error) {
	return scanUser(r.store.DB.QueryRowContext(ctx, userQuery+` WHERE lower(email) = lower(?)`, email))
}
func (r *UserRepository) ListMembers(ctx context.Context) ([]domain.User, error) {
	rows, err := r.store.DB.QueryContext(ctx, userQuery+` WHERE role=? ORDER BY display_name, email`, domain.RoleMember)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.User
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, user)
	}
	return result, rows.Err()
}
func (r *UserRepository) UpdateActive(ctx context.Context, id string, active bool, updatedAt string) error {
	_, err := r.store.DB.ExecContext(ctx, `UPDATE users SET is_active=?, updated_at=? WHERE id=?`, boolInt(active), updatedAt, id)
	return err
}

func (r *UserRepository) Update(ctx context.Context, id, email, displayName string, active bool, updatedAt string) error {
	_, err := r.store.DB.ExecContext(ctx, `UPDATE users SET email=?,display_name=?,is_active=?,updated_at=? WHERE id=?`, email, displayName, boolInt(active), updatedAt, id)
	return err
}

func (t *Tx) User(id string) (domain.User, error) {
	return scanUser(t.tx.QueryRow(userQuery+` WHERE id = ?`, id))
}
func (t *Tx) UserByEmail(email string) (domain.User, error) {
	return scanUser(t.tx.QueryRow(userQuery+` WHERE lower(email) = lower(?)`, email))
}

func (t *Tx) UpdateUser(id, email, displayName string, active bool, updatedAt string) error {
	result, err := t.tx.Exec(`UPDATE users SET email=?,display_name=?,is_active=?,updated_at=? WHERE id=?`, email, displayName, boolInt(active), updatedAt, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		return domain.ErrNotFound
	}
	return err
}

const userQuery = `SELECT id,email,display_name,role,is_active,created_at,updated_at FROM users`

type rowScanner interface{ Scan(...any) error }

func scanUser(row rowScanner) (domain.User, error) {
	var user domain.User
	var active int
	err := row.Scan(&user.ID, &user.Email, &user.DisplayName, &user.Role, &active, &user.CreatedAt, &user.UpdatedAt)
	if err == sql.ErrNoRows {
		return user, domain.ErrNotFound
	}
	if err != nil {
		return user, fmt.Errorf("scan user: %w", err)
	}
	user.IsActive = active == 1
	return user, nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
