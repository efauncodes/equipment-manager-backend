package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/efauncodes/equipment-manager-backend/domain"
	"github.com/efauncodes/equipment-manager-backend/repository"
)

const (
	TimestampLayout = "2006-01-02 15:04:05"
	DateLayout      = "2006-01-02"
)

type Clock func() time.Time
type Service struct {
	store *repository.Store
	now   Clock
}

func New(store *repository.Store) *Service {
	return &Service{store: store, now: func() time.Time { return time.Now().In(location()) }}
}
func NewWithClock(store *repository.Store, now Clock) *Service {
	if now == nil {
		return New(store)
	}
	return &Service{store: store, now: now}
}
func location() *time.Location {
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		return time.FixedZone("Europe/Berlin", 3600)
	}
	return loc
}
func (s *Service) timestamp() string  { return s.now().In(location()).Format(TimestampLayout) }
func (s *Service) dateNow() time.Time { return s.now().In(location()) }
func (s *Service) later(minutes int) string {
	return s.dateNow().Add(time.Duration(minutes) * time.Minute).Format(TimestampLayout)
}

type CreateEquipmentInput struct {
	SerialNumber, Type, Size string
	PurchaseDate             *string
	Manufacturer             string
}
type TokenResult struct {
	RawToken string
	Link     domain.MagicLink
}
type SessionResult struct {
	RawToken string
	Session  domain.Session
	User     domain.User
}

func (s *Service) CreateUser(ctx context.Context, email, displayName, role string, active bool) (domain.User, error) {
	email = domain.NormalizeEmail(email)
	displayName = strings.TrimSpace(displayName)
	if email == "" || displayName == "" || domain.ValidateRole(role) != nil {
		return domain.User{}, fmt.Errorf("%w: invalid user", domain.ErrInvalid)
	}
	if !active {
		active = false
	}
	now := s.timestamp()
	user := domain.User{ID: newID(), Email: email, DisplayName: displayName, Role: role, IsActive: active, CreatedAt: now, UpdatedAt: now}
	if err := s.store.Users().Create(ctx, user); err != nil {
		return domain.User{}, mapWriteError(err)
	}
	return user, nil
}

func (s *Service) SetUserActive(ctx context.Context, actorID, id string, active bool) error {
	actor, err := s.store.Users().Get(ctx, actorID)
	if err != nil {
		return err
	}
	if err := ensureAdmin(actor); err != nil {
		return err
	}
	if _, err := s.store.Users().Get(ctx, id); err != nil {
		return err
	}
	return s.store.Users().UpdateActive(ctx, id, active, s.timestamp())
}

func (s *Service) CreateEquipment(ctx context.Context, actorID string, input CreateEquipmentInput) (domain.Equipment, error) {
	item := domain.Equipment{ID: newID(), SerialNumber: domain.NormalizeSerial(input.SerialNumber), Type: strings.TrimSpace(input.Type), Size: strings.TrimSpace(input.Size), PurchaseDate: input.PurchaseDate, Manufacturer: strings.TrimSpace(input.Manufacturer), Status: domain.StatusOnStock}
	if item.SerialNumber == "" || item.Type == "" || item.Size == "" || item.Manufacturer == "" {
		return item, fmt.Errorf("%w: equipment fields are required", domain.ErrInvalid)
	}
	if item.PurchaseDate != nil {
		if _, err := time.Parse(DateLayout, *item.PurchaseDate); err != nil {
			return item, fmt.Errorf("%w: invalid purchase date", domain.ErrInvalid)
		}
	}
	now := s.timestamp()
	item.CreatedAt = now
	item.UpdatedAt = now
	err := s.store.InTx(ctx, func(tx *repository.Tx) error {
		actor, err := tx.User(actorID)
		if err != nil {
			return err
		}
		if err := ensureAdmin(actor); err != nil {
			return err
		}
		if err := tx.CreateEquipment(item); err != nil {
			return mapWriteError(err)
		}
		return tx.CreateHistory(domain.History{ID: newID(), EquipmentID: item.ID, EventType: domain.EventCreated, ToStatus: ptr(item.Status), ChangedByUserID: actor.ID, OccurredAt: now})
	})
	if err != nil {
		return domain.Equipment{}, err
	}
	return item, nil
}

func (s *Service) GetEquipment(ctx context.Context, id string) (domain.Equipment, error) {
	return s.store.Equipment().Get(ctx, id)
}
func (s *Service) ListEquipment(ctx context.Context, includeWrittenOff bool) ([]domain.Equipment, error) {
	return s.store.Equipment().List(ctx, includeWrittenOff)
}
func (s *Service) History(ctx context.Context, equipmentID string) ([]domain.History, error) {
	return s.store.History().ListByEquipment(ctx, equipmentID)
}

func (s *Service) Issue(ctx context.Context, equipmentID, memberID, adminID string) (domain.Issuance, error) {
	now := s.timestamp()
	var result domain.Issuance
	err := s.store.InTx(ctx, func(tx *repository.Tx) error {
		admin, err := tx.User(adminID)
		if err != nil {
			return err
		}
		if err = ensureAdmin(admin); err != nil {
			return err
		}
		member, err := tx.User(memberID)
		if err != nil {
			return err
		}
		if err = ensureMember(member); err != nil {
			return err
		}
		item, err := tx.Equipment(equipmentID)
		if err != nil {
			return err
		}
		if item.Status != domain.StatusOnStock {
			return fmt.Errorf("%w: equipment status is %s", domain.ErrUnavailable, item.Status)
		}
		result = domain.Issuance{ID: newID(), EquipmentID: equipmentID, MemberID: memberID, IssuedByUserID: adminID, ConfirmationStatus: domain.ConfirmationPending, CreatedAt: now, UpdatedAt: now}
		if err := tx.CreateIssuance(result); err != nil {
			return mapWriteError(err)
		}
		return nil
	})
	return result, err
}

func (s *Service) CancelIssuance(ctx context.Context, id, adminID string) error {
	now := s.timestamp()
	return s.store.InTx(ctx, func(tx *repository.Tx) error {
		admin, err := tx.User(adminID)
		if err != nil {
			return err
		}
		if err = ensureAdmin(admin); err != nil {
			return err
		}
		if _, err = tx.Issuance(id); err != nil {
			return err
		}
		return tx.CancelIssuance(id, now)
	})
}

func (s *Service) ConfirmIssuance(ctx context.Context, issuanceID, memberID string) (domain.Issuance, error) {
	var result domain.Issuance
	err := s.store.InTx(ctx, func(tx *repository.Tx) error {
		now := s.timestamp()
		i, err := tx.Issuance(issuanceID)
		if err != nil {
			return err
		}
		member, err := tx.User(memberID)
		if err != nil {
			return err
		}
		if err = ensureMember(member); err != nil {
			return err
		}
		if i.MemberID != memberID {
			return fmt.Errorf("%w: member mismatch", domain.ErrConflict)
		}
		if i.ConfirmationStatus != domain.ConfirmationPending || i.ClosedAt != nil {
			return domain.ErrConflict
		}
		item, err := tx.Equipment(i.EquipmentID)
		if err != nil {
			return err
		}
		if item.Status != domain.StatusOnStock {
			return fmt.Errorf("%w: equipment status is %s", domain.ErrUnavailable, item.Status)
		}
		if err := tx.ConfirmIssuance(issuanceID, now, now); err != nil {
			return err
		}
		if err := tx.UpdateEquipmentStatus(item.ID, domain.StatusIssued, now); err != nil {
			return err
		}
		if err := tx.CreateHistory(domain.History{ID: newID(), EquipmentID: item.ID, EventType: domain.EventIssued, FromStatus: ptr(item.Status), ToStatus: ptr(domain.StatusIssued), IssuanceID: ptr(i.ID), ChangedByUserID: memberID, OccurredAt: now}); err != nil {
			return err
		}
		result, err = tx.Issuance(issuanceID)
		return err
	})
	return result, err
}

func (s *Service) StartReturn(ctx context.Context, issuanceID, adminID string) (domain.Return, error) {
	var result domain.Return
	now := s.timestamp()
	err := s.store.InTx(ctx, func(tx *repository.Tx) error {
		admin, err := tx.User(adminID)
		if err != nil {
			return err
		}
		if err = ensureAdmin(admin); err != nil {
			return err
		}
		i, err := tx.Issuance(issuanceID)
		if err != nil {
			return err
		}
		if i.ConfirmationStatus != domain.ConfirmationConfirmed || i.ClosedAt != nil {
			return domain.ErrConflict
		}
		if _, err = tx.ReturnByIssuance(issuanceID); err == nil {
			return domain.ErrConflict
		} else if !errors.Is(err, domain.ErrNotFound) {
			return err
		}
		result = domain.Return{ID: newID(), IssuanceID: issuanceID, InitiatedByUserID: adminID, ConfirmationStatus: domain.ConfirmationPending, CreatedAt: now, UpdatedAt: now}
		return tx.CreateReturn(result)
	})
	return result, err
}

func (s *Service) CancelReturn(ctx context.Context, returnID, adminID string) error {
	now := s.timestamp()
	return s.store.InTx(ctx, func(tx *repository.Tx) error {
		admin, err := tx.User(adminID)
		if err != nil {
			return err
		}
		if err = ensureAdmin(admin); err != nil {
			return err
		}
		if _, err = tx.Return(returnID); err != nil {
			return err
		}
		return tx.CancelReturn(returnID, now)
	})
}

func (s *Service) ConfirmReturn(ctx context.Context, returnID, memberID string) (domain.Return, error) {
	var result domain.Return
	err := s.store.InTx(ctx, func(tx *repository.Tx) error {
		now := s.timestamp()
		r, err := tx.Return(returnID)
		if err != nil {
			return err
		}
		i, err := tx.Issuance(r.IssuanceID)
		if err != nil {
			return err
		}
		member, err := tx.User(memberID)
		if err != nil {
			return err
		}
		if err = ensureMember(member); err != nil {
			return err
		}
		if member.ID != i.MemberID {
			return fmt.Errorf("%w: member mismatch", domain.ErrConflict)
		}
		if r.ConfirmationStatus != domain.ConfirmationPending || i.ClosedAt != nil {
			return domain.ErrConflict
		}
		item, err := tx.Equipment(i.EquipmentID)
		if err != nil {
			return err
		}
		if item.Status != domain.StatusIssued {
			return fmt.Errorf("%w: equipment status is %s", domain.ErrConflict, item.Status)
		}
		if err := tx.ConfirmReturn(returnID, now, now, now); err != nil {
			return err
		}
		if err := tx.CloseIssuance(i.ID, now, now, "returned", now); err != nil {
			return err
		}
		if err := tx.UpdateEquipmentStatus(item.ID, domain.StatusOnStock, now); err != nil {
			return err
		}
		if err := tx.CreateHistory(domain.History{ID: newID(), EquipmentID: item.ID, EventType: domain.EventReturned, FromStatus: ptr(item.Status), ToStatus: ptr(domain.StatusOnStock), IssuanceID: ptr(i.ID), ReturnID: ptr(r.ID), ChangedByUserID: memberID, OccurredAt: now}); err != nil {
			return err
		}
		result, err = tx.Return(returnID)
		return err
	})
	return result, err
}

func (s *Service) ChangeStatus(ctx context.Context, equipmentID, target, adminID, note string) (domain.Equipment, error) {
	if target != domain.StatusLost && target != domain.StatusDamaged {
		return domain.Equipment{}, fmt.Errorf("%w: only lost or damaged may be set manually", domain.ErrInvalid)
	}
	var result domain.Equipment
	err := s.store.InTx(ctx, func(tx *repository.Tx) error {
		now := s.timestamp()
		admin, err := tx.User(adminID)
		if err != nil {
			return err
		}
		if err = ensureAdmin(admin); err != nil {
			return err
		}
		item, err := tx.Equipment(equipmentID)
		if err != nil {
			return err
		}
		if item.Status != domain.StatusOnStock && item.Status != domain.StatusIssued {
			return domain.ErrConflict
		}
		if item.Status == target {
			return domain.ErrConflict
		}
		var issuanceID *string
		if item.Status == domain.StatusIssued {
			active, err := tx.ActiveIssuanceForEquipment(item.ID)
			if err != nil {
				return err
			}
			issuanceID = ptr(active.ID)
			if err := tx.CloseIssuance(active.ID, "", now, target, now); err != nil {
				return err
			}
			if pendingReturn, err := tx.ReturnByIssuance(active.ID); err == nil {
				if pendingReturn.ConfirmationStatus == domain.ConfirmationPending {
					if err := tx.CancelReturn(pendingReturn.ID, now); err != nil {
						return err
					}
				}
			} else if !errors.Is(err, domain.ErrNotFound) {
				return err
			}
		}
		if err := tx.UpdateEquipmentStatus(item.ID, target, now); err != nil {
			return err
		}
		if err := tx.CreateHistory(domain.History{ID: newID(), EquipmentID: item.ID, EventType: domain.EventStatusChanged, FromStatus: ptr(item.Status), ToStatus: ptr(target), IssuanceID: issuanceID, ChangedByUserID: adminID, OccurredAt: now, Note: optional(note)}); err != nil {
			return err
		}
		result, err = tx.Equipment(item.ID)
		return err
	})
	return result, err
}

func (s *Service) WriteOff(ctx context.Context, equipmentID, adminID, note string) (domain.Equipment, error) {
	var result domain.Equipment
	err := s.store.InTx(ctx, func(tx *repository.Tx) error {
		now := s.timestamp()
		admin, err := tx.User(adminID)
		if err != nil {
			return err
		}
		if err = ensureAdmin(admin); err != nil {
			return err
		}
		item, err := tx.Equipment(equipmentID)
		if err != nil {
			return err
		}
		if item.Status != domain.StatusLost && item.Status != domain.StatusDamaged {
			return fmt.Errorf("%w: only lost or damaged equipment can be written off", domain.ErrConflict)
		}
		if err := tx.UpdateEquipmentStatus(item.ID, domain.StatusWrittenOff, now); err != nil {
			return err
		}
		if err := tx.CreateHistory(domain.History{ID: newID(), EquipmentID: item.ID, EventType: domain.EventWrittenOff, FromStatus: ptr(item.Status), ToStatus: ptr(domain.StatusWrittenOff), ChangedByUserID: adminID, OccurredAt: now, Note: optional(note)}); err != nil {
			return err
		}
		result, err = tx.Equipment(item.ID)
		return err
	})
	return result, err
}

func (s *Service) CreateLoginLink(ctx context.Context, userID string) (TokenResult, error) {
	return s.createLink(ctx, userID, domain.PurposeLogin, "", "")
}
func (s *Service) CreateIssuanceConfirmationLink(ctx context.Context, issuanceID string) (TokenResult, error) {
	var result TokenResult
	err := s.store.InTx(ctx, func(tx *repository.Tx) error {
		i, err := tx.Issuance(issuanceID)
		if err != nil {
			return err
		}
		if i.ConfirmationStatus != domain.ConfirmationPending || i.ClosedAt != nil {
			return domain.ErrConflict
		}
		return s.createConfirmationInTx(tx, i.MemberID, issuanceID, "", &result)
	})
	return result, err
}
func (s *Service) CreateReturnConfirmationLink(ctx context.Context, returnID string) (TokenResult, error) {
	var result TokenResult
	err := s.store.InTx(ctx, func(tx *repository.Tx) error {
		r, err := tx.Return(returnID)
		if err != nil {
			return err
		}
		if r.ConfirmationStatus != domain.ConfirmationPending {
			return domain.ErrConflict
		}
		i, err := tx.Issuance(r.IssuanceID)
		if err != nil {
			return err
		}
		return s.createConfirmationInTx(tx, i.MemberID, "", returnID, &result)
	})
	return result, err
}

func (s *Service) createLink(ctx context.Context, userID, purpose, issuanceID, returnID string) (TokenResult, error) {
	var result TokenResult
	err := s.store.InTx(ctx, func(tx *repository.Tx) error {
		user, err := tx.User(userID)
		if err != nil {
			return err
		}
		if err = ensureActive(user); err != nil {
			return err
		}
		raw := newToken()
		now := s.timestamp()
		link := domain.MagicLink{ID: newID(), UserID: userID, TokenHash: hash(raw), Purpose: purpose, ExpiresAt: s.later(10), CreatedAt: now}
		if err := tx.CreateMagicLink(link); err != nil {
			return mapWriteError(err)
		}
		result = TokenResult{RawToken: raw, Link: link}
		return nil
	})
	return result, err
}
func (s *Service) createConfirmationInTx(tx *repository.Tx, userID, issuanceID, returnID string, result *TokenResult) error {
	user, err := tx.User(userID)
	if err != nil {
		return err
	}
	if err = ensureActive(user); err != nil {
		return err
	}
	raw := newToken()
	now := s.timestamp()
	link := domain.MagicLink{ID: newID(), UserID: userID, TokenHash: hash(raw), Purpose: domain.PurposeConfirm, IssuanceID: optional(issuanceID), ReturnID: optional(returnID), ExpiresAt: s.later(10), CreatedAt: now}
	if err := tx.CreateMagicLink(link); err != nil {
		return mapWriteError(err)
	}
	*result = TokenResult{RawToken: raw, Link: link}
	return nil
}

func (s *Service) RedeemLoginLink(ctx context.Context, raw string) (SessionResult, error) {
	var result SessionResult
	err := s.store.InTx(ctx, func(tx *repository.Tx) error {
		now := s.timestamp()
		link, err := tx.MagicLinkByHash(hash(raw))
		if err != nil {
			return err
		}
		if err = validateLink(link, now, domain.PurposeLogin); err != nil {
			return err
		}
		user, err := tx.User(link.UserID)
		if err != nil {
			return err
		}
		if err = ensureActive(user); err != nil {
			return err
		}
		if err := tx.MarkMagicLinkUsed(link.ID, now); err != nil {
			return err
		}
		sessionRaw := newToken()
		session := domain.Session{ID: newID(), UserID: user.ID, TokenHash: hash(sessionRaw), ExpiresAt: s.later(10), CreatedAt: now}
		if err := tx.CreateSession(session); err != nil {
			return mapWriteError(err)
		}
		result = SessionResult{RawToken: sessionRaw, Session: session, User: user}
		return nil
	})
	return result, err
}

func (s *Service) RedeemConfirmationLink(ctx context.Context, raw string) (string, error) {
	var event string
	err := s.store.InTx(ctx, func(tx *repository.Tx) error {
		now := s.timestamp()
		link, err := tx.MagicLinkByHash(hash(raw))
		if err != nil {
			return err
		}
		if err = validateLink(link, now, domain.PurposeConfirm); err != nil {
			return err
		}
		user, err := tx.User(link.UserID)
		if err != nil {
			return err
		}
		if err = ensureActive(user); err != nil {
			return err
		}
		if err := tx.MarkMagicLinkUsed(link.ID, now); err != nil {
			return err
		}
		if link.IssuanceID != nil {
			i, err := tx.Issuance(*link.IssuanceID)
			if err != nil {
				return err
			}
			if i.MemberID != user.ID {
				return domain.ErrConflict
			}
			item, err := tx.Equipment(i.EquipmentID)
			if err != nil {
				return err
			}
			if item.Status != domain.StatusOnStock {
				return domain.ErrUnavailable
			}
			if err := tx.ConfirmIssuance(i.ID, now, now); err != nil {
				return err
			}
			if err := tx.UpdateEquipmentStatus(item.ID, domain.StatusIssued, now); err != nil {
				return err
			}
			if err := tx.CreateHistory(domain.History{ID: newID(), EquipmentID: item.ID, EventType: domain.EventIssued, FromStatus: ptr(item.Status), ToStatus: ptr(domain.StatusIssued), IssuanceID: ptr(i.ID), ChangedByUserID: user.ID, OccurredAt: now}); err != nil {
				return err
			}
			event = domain.EventIssued
			return nil
		}
		r, err := tx.Return(*link.ReturnID)
		if err != nil {
			return err
		}
		i, err := tx.Issuance(r.IssuanceID)
		if err != nil {
			return err
		}
		if i.MemberID != user.ID {
			return domain.ErrConflict
		}
		item, err := tx.Equipment(i.EquipmentID)
		if err != nil {
			return err
		}
		if item.Status != domain.StatusIssued {
			return domain.ErrConflict
		}
		if err := tx.ConfirmReturn(r.ID, now, now, now); err != nil {
			return err
		}
		if err := tx.CloseIssuance(i.ID, now, now, "returned", now); err != nil {
			return err
		}
		if err := tx.UpdateEquipmentStatus(item.ID, domain.StatusOnStock, now); err != nil {
			return err
		}
		if err := tx.CreateHistory(domain.History{ID: newID(), EquipmentID: item.ID, EventType: domain.EventReturned, FromStatus: ptr(item.Status), ToStatus: ptr(domain.StatusOnStock), IssuanceID: ptr(i.ID), ReturnID: ptr(r.ID), ChangedByUserID: user.ID, OccurredAt: now}); err != nil {
			return err
		}
		event = domain.EventReturned
		return nil
	})
	return event, err
}

func (s *Service) ValidateSession(ctx context.Context, raw string) (domain.User, error) {
	session, err := s.store.Sessions().GetByHash(ctx, hash(raw))
	if err != nil {
		return domain.User{}, err
	}
	if session.RevokedAt != nil || session.ExpiresAt <= s.timestamp() {
		return domain.User{}, domain.ErrExpired
	}
	user, err := s.store.Users().Get(ctx, session.UserID)
	if err != nil {
		return domain.User{}, err
	}
	if err = ensureActive(user); err != nil {
		return domain.User{}, err
	}
	return user, nil
}
func (s *Service) RevokeSession(ctx context.Context, raw string) error {
	session, err := s.store.Sessions().GetByHash(ctx, hash(raw))
	if err != nil {
		return err
	}
	return s.store.Sessions().Revoke(ctx, session.ID, s.timestamp())
}

func ensureAdmin(user domain.User) error {
	if err := ensureActive(user); err != nil {
		return err
	}
	if user.Role != domain.RoleAdmin {
		return fmt.Errorf("%w: admin role required", domain.ErrConflict)
	}
	return nil
}
func ensureMember(user domain.User) error {
	if err := ensureActive(user); err != nil {
		return err
	}
	if user.Role != domain.RoleMember {
		return fmt.Errorf("%w: member role required", domain.ErrConflict)
	}
	return nil
}
func ensureActive(user domain.User) error {
	if !user.IsActive {
		return domain.ErrInactive
	}
	return nil
}
func validateLink(link domain.MagicLink, now, purpose string) error {
	if link.Purpose != purpose {
		return domain.ErrConflict
	}
	if link.UsedAt != nil {
		return domain.ErrUsed
	}
	if link.ExpiresAt <= now {
		return domain.ErrExpired
	}
	return nil
}
func mapWriteError(err error) error {
	if err == nil {
		return nil
	}
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "unique") || strings.Contains(lower, "constraint") || strings.Contains(lower, "locked") {
		return fmt.Errorf("%w: %v", domain.ErrConflict, err)
	}
	return err
}
func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
func newToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func hash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
func ptr(value string) *string { return &value }
func optional(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
