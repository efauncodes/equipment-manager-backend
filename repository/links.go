package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/efauncodes/equipment-manager-backend/domain"
)

type MagicLinkRepository struct{ store *Store }

func (s *Store) MagicLinks() *MagicLinkRepository { return &MagicLinkRepository{store: s} }
func (t *Tx) CreateMagicLink(value domain.MagicLink) error {
	_, err := t.tx.Exec(`INSERT INTO magic_links(id,user_id,token_hash,purpose,issuance_id,return_id,expires_at,used_at,created_at) VALUES(?,?,?,?,?,?,?,?,?)`, value.ID, value.UserID, value.TokenHash, value.Purpose, valueOrNil(value.IssuanceID), valueOrNil(value.ReturnID), value.ExpiresAt, valueOrNil(value.UsedAt), value.CreatedAt)
	return err
}
func (t *Tx) MagicLinkByHash(hash string) (domain.MagicLink, error) {
	return scanMagicLink(t.tx.QueryRow(magicLinkQuery+` WHERE token_hash=?`, hash))
}
func (t *Tx) MarkMagicLinkUsed(id, usedAt string) error {
	result, err := t.tx.Exec(`UPDATE magic_links SET used_at=? WHERE id=? AND used_at IS NULL`, usedAt, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		return domain.ErrUsed
	}
	return err
}

const magicLinkQuery = `SELECT id,user_id,token_hash,purpose,issuance_id,return_id,expires_at,used_at,created_at FROM magic_links`

func scanMagicLink(row rowScanner) (domain.MagicLink, error) {
	var v domain.MagicLink
	var issuance, ret, used sql.NullString
	err := row.Scan(&v.ID, &v.UserID, &v.TokenHash, &v.Purpose, &issuance, &ret, &v.ExpiresAt, &used, &v.CreatedAt)
	if err == sql.ErrNoRows {
		return v, domain.ErrNotFound
	}
	if err != nil {
		return v, fmt.Errorf("scan magic link: %w", err)
	}
	v.IssuanceID = nullableString(issuance)
	v.ReturnID = nullableString(ret)
	v.UsedAt = nullableString(used)
	return v, nil
}

type SessionRepository struct{ store *Store }

func (s *Store) Sessions() *SessionRepository { return &SessionRepository{store: s} }
func (t *Tx) CreateSession(value domain.Session) error {
	_, err := t.tx.Exec(`INSERT INTO sessions(id,user_id,token_hash,expires_at,revoked_at,created_at) VALUES(?,?,?,?,?,?)`, value.ID, value.UserID, value.TokenHash, value.ExpiresAt, valueOrNil(value.RevokedAt), value.CreatedAt)
	return err
}
func (r *SessionRepository) GetByHash(ctx context.Context, hash string) (domain.Session, error) {
	return scanSession(r.store.DB.QueryRowContext(ctx, sessionQuery+` WHERE token_hash=?`, hash))
}
func (r *SessionRepository) Revoke(ctx context.Context, id, at string) error {
	_, err := r.store.DB.ExecContext(ctx, `UPDATE sessions SET revoked_at=? WHERE id=?`, at, id)
	return err
}

const sessionQuery = `SELECT id,user_id,token_hash,expires_at,revoked_at,created_at FROM sessions`

func scanSession(row rowScanner) (domain.Session, error) {
	var v domain.Session
	var revoked sql.NullString
	err := row.Scan(&v.ID, &v.UserID, &v.TokenHash, &v.ExpiresAt, &revoked, &v.CreatedAt)
	if err == sql.ErrNoRows {
		return v, domain.ErrNotFound
	}
	if err != nil {
		return v, fmt.Errorf("scan session: %w", err)
	}
	v.RevokedAt = nullableString(revoked)
	return v, nil
}
