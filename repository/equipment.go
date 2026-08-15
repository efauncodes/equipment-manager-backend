package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/efauncodes/equipment-manager-backend/domain"
)

type EquipmentRepository struct{ store *Store }

func (s *Store) Equipment() *EquipmentRepository { return &EquipmentRepository{store: s} }

func (r *EquipmentRepository) Create(ctx context.Context, item domain.Equipment) error {
	return createEquipment(ctx, r.store.DB, item)
}
func (r *EquipmentRepository) Get(ctx context.Context, id string) (domain.Equipment, error) {
	return scanEquipment(r.store.DB.QueryRowContext(ctx, equipmentQuery+` WHERE id = ?`, id))
}
func (r *EquipmentRepository) List(ctx context.Context, includeWrittenOff bool) ([]domain.Equipment, error) {
	query := equipmentQuery
	if !includeWrittenOff {
		query += ` WHERE status <> 'written_off'`
	}
	query += ` ORDER BY serial_number`
	rows, err := r.store.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []domain.Equipment
	for rows.Next() {
		item, err := scanEquipment(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *EquipmentRepository) Update(ctx context.Context, item domain.Equipment) error {
	_, err := r.store.DB.ExecContext(ctx, `UPDATE equipment SET serial_number=?,type=?,size=?,purchase_date=?,manufacturer=?,updated_at=? WHERE id=?`, item.SerialNumber, item.Type, item.Size, valueOrNil(item.PurchaseDate), item.Manufacturer, item.UpdatedAt, item.ID)
	return err
}

func (t *Tx) Equipment(id string) (domain.Equipment, error) {
	return scanEquipment(t.tx.QueryRow(equipmentQuery+` WHERE id = ?`, id))
}
func (t *Tx) CreateEquipment(item domain.Equipment) error {
	return createEquipment(context.Background(), t.tx, item)
}
func (t *Tx) UpdateEquipmentStatus(id, status, updatedAt string) error {
	result, err := t.tx.Exec(`UPDATE equipment SET status=?, updated_at=? WHERE id=?`, status, updatedAt, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		return domain.ErrNotFound
	}
	return err
}

func (t *Tx) UpdateEquipment(item domain.Equipment) error {
	result, err := t.tx.Exec(`UPDATE equipment SET serial_number=?,type=?,size=?,purchase_date=?,manufacturer=?,updated_at=? WHERE id=?`, item.SerialNumber, item.Type, item.Size, valueOrNil(item.PurchaseDate), item.Manufacturer, item.UpdatedAt, item.ID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		return domain.ErrNotFound
	}
	return err
}

const equipmentQuery = `SELECT id,serial_number,type,size,purchase_date,manufacturer,status,created_at,updated_at FROM equipment`

func createEquipment(ctx context.Context, exec interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, item domain.Equipment) error {
	_, err := exec.ExecContext(ctx, `INSERT INTO equipment(id,serial_number,type,size,purchase_date,manufacturer,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`, item.ID, item.SerialNumber, item.Type, item.Size, valueOrNil(item.PurchaseDate), item.Manufacturer, item.Status, item.CreatedAt, item.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create equipment: %w", err)
	}
	return nil
}

func scanEquipment(row rowScanner) (domain.Equipment, error) {
	var item domain.Equipment
	var purchase sql.NullString
	err := row.Scan(&item.ID, &item.SerialNumber, &item.Type, &item.Size, &purchase, &item.Manufacturer, &item.Status, &item.CreatedAt, &item.UpdatedAt)
	if err == sql.ErrNoRows {
		return item, domain.ErrNotFound
	}
	if err != nil {
		return item, fmt.Errorf("scan equipment: %w", err)
	}
	item.PurchaseDate = nullableString(purchase)
	return item, nil
}
