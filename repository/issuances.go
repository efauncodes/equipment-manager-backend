package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/efauncodes/equipment-manager-backend/domain"
)

type IssuanceRepository struct{ store *Store }

func (s *Store) Issuances() *IssuanceRepository { return &IssuanceRepository{store: s} }
func (r *IssuanceRepository) Get(ctx context.Context, id string) (domain.Issuance, error) {
	return scanIssuance(r.store.DB.QueryRowContext(ctx, issuanceQuery+` WHERE id=?`, id))
}
func (t *Tx) Issuance(id string) (domain.Issuance, error) {
	return scanIssuance(t.tx.QueryRow(issuanceQuery+` WHERE id=?`, id))
}
func (t *Tx) ActiveIssuanceForEquipment(id string) (domain.Issuance, error) {
	return scanIssuance(t.tx.QueryRow(issuanceQuery+` WHERE equipment_id=? AND confirmation_status=? AND closed_at IS NULL`, id, domain.ConfirmationConfirmed))
}
func (t *Tx) CreateIssuance(i domain.Issuance) error {
	_, err := t.tx.Exec(`INSERT INTO issuances(id,equipment_id,member_id,issued_by_user_id,issued_at,confirmation_status,member_confirmed_at,returned_at,closed_at,closure_reason,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, i.ID, i.EquipmentID, i.MemberID, i.IssuedByUserID, valueOrNil(i.IssuedAt), i.ConfirmationStatus, valueOrNil(i.MemberConfirmedAt), valueOrNil(i.ReturnedAt), valueOrNil(i.ClosedAt), valueOrNil(i.ClosureReason), i.CreatedAt, i.UpdatedAt)
	return err
}
func (t *Tx) ConfirmIssuance(id, issuedAt, confirmedAt string) error {
	result, err := t.tx.Exec(`UPDATE issuances SET issued_at=?,confirmation_status=?,member_confirmed_at=?,updated_at=? WHERE id=? AND confirmation_status=? AND closed_at IS NULL`, issuedAt, domain.ConfirmationConfirmed, confirmedAt, confirmedAt, id, domain.ConfirmationPending)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		return domain.ErrConflict
	}
	return err
}
func (t *Tx) CancelIssuance(id, updatedAt string) error {
	result, err := t.tx.Exec(`UPDATE issuances SET confirmation_status=?,updated_at=? WHERE id=? AND confirmation_status=? AND closed_at IS NULL`, domain.ConfirmationCancelled, updatedAt, id, domain.ConfirmationPending)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		return domain.ErrConflict
	}
	return err
}
func (t *Tx) CloseIssuance(id, returnedAt, closedAt, reason, updatedAt string) error {
	result, err := t.tx.Exec(`UPDATE issuances SET returned_at=?,closed_at=?,closure_reason=?,updated_at=? WHERE id=? AND confirmation_status=? AND closed_at IS NULL`, valueOrNil(nonEmpty(returnedAt)), closedAt, reason, updatedAt, id, domain.ConfirmationConfirmed)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		return domain.ErrConflict
	}
	return err
}

const issuanceQuery = `SELECT id,equipment_id,member_id,issued_by_user_id,issued_at,confirmation_status,member_confirmed_at,returned_at,closed_at,closure_reason,created_at,updated_at FROM issuances`

func scanIssuance(row rowScanner) (domain.Issuance, error) {
	var i domain.Issuance
	var issued, confirmed, returned, closed, reason sql.NullString
	err := row.Scan(&i.ID, &i.EquipmentID, &i.MemberID, &i.IssuedByUserID, &issued, &i.ConfirmationStatus, &confirmed, &returned, &closed, &reason, &i.CreatedAt, &i.UpdatedAt)
	if err == sql.ErrNoRows {
		return i, domain.ErrNotFound
	}
	if err != nil {
		return i, fmt.Errorf("scan issuance: %w", err)
	}
	i.IssuedAt = nullableString(issued)
	i.MemberConfirmedAt = nullableString(confirmed)
	i.ReturnedAt = nullableString(returned)
	i.ClosedAt = nullableString(closed)
	i.ClosureReason = nullableString(reason)
	return i, nil
}
func nonEmpty(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
