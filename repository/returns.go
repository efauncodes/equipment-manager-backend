package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/efauncodes/equipment-manager-backend/domain"
)

type ReturnRepository struct{ store *Store }

func (s *Store) Returns() *ReturnRepository { return &ReturnRepository{store: s} }
func (r *ReturnRepository) Get(ctx context.Context, id string) (domain.Return, error) {
	return scanReturn(r.store.DB.QueryRowContext(ctx, returnQuery+` WHERE id=?`, id))
}
func (r *ReturnRepository) GetByIssuance(ctx context.Context, id string) (domain.Return, error) {
	return scanReturn(r.store.DB.QueryRowContext(ctx, returnQuery+` WHERE issuance_id=?`, id))
}
func (t *Tx) Return(id string) (domain.Return, error) {
	return scanReturn(t.tx.QueryRow(returnQuery+` WHERE id=?`, id))
}
func (t *Tx) ReturnByIssuance(id string) (domain.Return, error) {
	return scanReturn(t.tx.QueryRow(returnQuery+` WHERE issuance_id=?`, id))
}
func (t *Tx) CreateReturn(value domain.Return) error {
	_, err := t.tx.Exec(`INSERT INTO returns(id,issuance_id,initiated_by_user_id,returned_at,confirmation_status,member_confirmed_at,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)`, value.ID, value.IssuanceID, value.InitiatedByUserID, valueOrNil(value.ReturnedAt), value.ConfirmationStatus, valueOrNil(value.MemberConfirmedAt), value.CreatedAt, value.UpdatedAt)
	return err
}
func (t *Tx) ConfirmReturn(id, returnedAt, confirmedAt, updatedAt string) error {
	result, err := t.tx.Exec(`UPDATE returns SET returned_at=?,confirmation_status=?,member_confirmed_at=?,updated_at=? WHERE id=? AND confirmation_status=?`, returnedAt, domain.ConfirmationConfirmed, confirmedAt, updatedAt, id, domain.ConfirmationPending)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		return domain.ErrConflict
	}
	return err
}
func (t *Tx) CancelReturn(id, updatedAt string) error {
	result, err := t.tx.Exec(`UPDATE returns SET confirmation_status=?,updated_at=? WHERE id=? AND confirmation_status=?`, domain.ConfirmationCancelled, updatedAt, id, domain.ConfirmationPending)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		return domain.ErrConflict
	}
	return err
}

const returnQuery = `SELECT id,issuance_id,initiated_by_user_id,returned_at,confirmation_status,member_confirmed_at,created_at,updated_at FROM returns`

func scanReturn(row rowScanner) (domain.Return, error) {
	var value domain.Return
	var returned, confirmed sql.NullString
	err := row.Scan(&value.ID, &value.IssuanceID, &value.InitiatedByUserID, &returned, &value.ConfirmationStatus, &confirmed, &value.CreatedAt, &value.UpdatedAt)
	if err == sql.ErrNoRows {
		return value, domain.ErrNotFound
	}
	if err != nil {
		return value, fmt.Errorf("scan return: %w", err)
	}
	value.ReturnedAt = nullableString(returned)
	value.MemberConfirmedAt = nullableString(confirmed)
	return value, nil
}
