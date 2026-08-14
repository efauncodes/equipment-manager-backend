package domain

import (
	"errors"
	"fmt"
	"strings"
)

const (
	RoleAdmin  = "admin"
	RoleMember = "mitglied"

	StatusOnStock    = "on_stock"
	StatusIssued     = "issued"
	StatusLost       = "lost"
	StatusDamaged    = "damaged"
	StatusWrittenOff = "written_off"

	ConfirmationPending   = "pending_confirmation"
	ConfirmationConfirmed = "confirmed"
	ConfirmationCancelled = "cancelled"

	EventCreated       = "created"
	EventStatusChanged = "status_changed"
	EventIssued        = "issued"
	EventReturned      = "returned"
	EventWrittenOff    = "written_off"

	PurposeLogin   = "login"
	PurposeConfirm = "confirm"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrConflict      = errors.New("conflict")
	ErrInvalid       = errors.New("invalid domain value")
	ErrInactive      = errors.New("user is inactive")
	ErrExpired       = errors.New("token expired")
	ErrUsed          = errors.New("token already used")
	ErrUnavailable   = errors.New("equipment unavailable")
	ErrAlreadyClosed = errors.New("operation already closed")
)

type User struct {
	ID          string
	Email       string
	DisplayName string
	Role        string
	IsActive    bool
	CreatedAt   string
	UpdatedAt   string
}

type Equipment struct {
	ID           string
	SerialNumber string
	Type         string
	Size         string
	PurchaseDate *string
	Manufacturer string
	Status       string
	CreatedAt    string
	UpdatedAt    string
}

type Issuance struct {
	ID                 string
	EquipmentID        string
	MemberID           string
	IssuedByUserID     string
	IssuedAt           *string
	ConfirmationStatus string
	MemberConfirmedAt  *string
	ReturnedAt         *string
	ClosedAt           *string
	ClosureReason      *string
	CreatedAt          string
	UpdatedAt          string
}

type Return struct {
	ID                 string
	IssuanceID         string
	InitiatedByUserID  string
	ReturnedAt         *string
	ConfirmationStatus string
	MemberConfirmedAt  *string
	CreatedAt          string
	UpdatedAt          string
}

type MagicLink struct {
	ID         string
	UserID     string
	TokenHash  string
	Purpose    string
	IssuanceID *string
	ReturnID   *string
	ExpiresAt  string
	UsedAt     *string
	CreatedAt  string
}

type Session struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt string
	RevokedAt *string
	CreatedAt string
}

type History struct {
	ID              string
	EquipmentID     string
	EventType       string
	FromStatus      *string
	ToStatus        *string
	IssuanceID      *string
	ReturnID        *string
	ChangedByUserID string
	OccurredAt      string
	Note            *string
}

func NormalizeEmail(value string) string  { return strings.ToLower(strings.TrimSpace(value)) }
func NormalizeSerial(value string) string { return strings.ToLower(strings.TrimSpace(value)) }

func ValidateRole(value string) error {
	if value != RoleAdmin && value != RoleMember {
		return fmt.Errorf("%w: role %q", ErrInvalid, value)
	}
	return nil
}

func ValidateStatus(value string) error {
	switch value {
	case StatusOnStock, StatusIssued, StatusLost, StatusDamaged, StatusWrittenOff:
		return nil
	}
	return fmt.Errorf("%w: status %q", ErrInvalid, value)
}

func ValidateConfirmation(value string) error {
	switch value {
	case ConfirmationPending, ConfirmationConfirmed, ConfirmationCancelled:
		return nil
	}
	return fmt.Errorf("%w: confirmation status %q", ErrInvalid, value)
}
