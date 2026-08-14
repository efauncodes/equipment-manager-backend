package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/efauncodes/equipment-manager-backend/domain"
)

type HistoryRepository struct{ store *Store }

func (s *Store) History() *HistoryRepository { return &HistoryRepository{store: s} }
func (r *HistoryRepository) ListByEquipment(ctx context.Context, equipmentID string) ([]domain.History, error) {
	return listHistory(ctx, r.store.DB, equipmentID)
}
func (t *Tx) CreateHistory(value domain.History) error {
	_, err := t.tx.Exec(`INSERT INTO equipment_history(id,equipment_id,event_type,from_status,to_status,issuance_id,return_id,changed_by_user_id,occurred_at,note) VALUES(?,?,?,?,?,?,?,?,?,?)`, value.ID, value.EquipmentID, value.EventType, valueOrNil(value.FromStatus), valueOrNil(value.ToStatus), valueOrNil(value.IssuanceID), valueOrNil(value.ReturnID), value.ChangedByUserID, value.OccurredAt, valueOrNil(value.Note))
	return err
}
func listHistory(ctx context.Context, q interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, equipmentID string) ([]domain.History, error) {
	rows, err := q.QueryContext(ctx, `SELECT id,equipment_id,event_type,from_status,to_status,issuance_id,return_id,changed_by_user_id,occurred_at,note FROM equipment_history WHERE equipment_id=? ORDER BY occurred_at,id`, equipmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.History
	for rows.Next() {
		var v domain.History
		var from, to, issuance, ret, note sql.NullString
		if err := rows.Scan(&v.ID, &v.EquipmentID, &v.EventType, &from, &to, &issuance, &ret, &v.ChangedByUserID, &v.OccurredAt, &note); err != nil {
			return nil, fmt.Errorf("scan history: %w", err)
		}
		v.FromStatus = nullableString(from)
		v.ToStatus = nullableString(to)
		v.IssuanceID = nullableString(issuance)
		v.ReturnID = nullableString(ret)
		v.Note = nullableString(note)
		result = append(result, v)
	}
	return result, rows.Err()
}
