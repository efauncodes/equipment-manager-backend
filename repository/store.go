package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

type Store struct{ DB *sql.DB }
type Tx struct{ tx *sql.Tx }

func NewStore(db *sql.DB) *Store { return &Store{DB: db} }

func (s *Store) InTx(ctx context.Context, fn func(*Tx) error) error {
	var last error
	for attempt := 0; attempt < 5; attempt++ {
		tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
		if err != nil {
			last = err
		} else {
			err = fn(&Tx{tx: tx})
			if err == nil {
				err = tx.Commit()
			} else {
				_ = tx.Rollback()
			}
			if err == nil {
				return nil
			}
			last = err
		}
		if !isBusy(err) || attempt == 4 {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 15 * time.Millisecond):
		}
	}
	return last
}

func isBusy(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "database is locked") || strings.Contains(s, "database table is locked") || strings.Contains(s, "busy")
}

func nullableString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	v := value.String
	return &v
}

func valueOrNil(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}
