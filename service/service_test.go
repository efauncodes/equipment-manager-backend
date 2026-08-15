package service

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/efauncodes/equipment-manager-backend/db"
	"github.com/efauncodes/equipment-manager-backend/domain"
	"github.com/efauncodes/equipment-manager-backend/repository"
)

type fixture struct {
	svc                  *Service
	database             interface{ Close() error }
	admin, member, other domain.User
	equipment            domain.Equipment
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "equipment.db"))
	if err != nil {
		t.Fatal(err)
	}
	store := repository.NewStore(database)
	clock := time.Date(2026, 8, 14, 10, 0, 0, 0, time.FixedZone("Europe/Berlin", 2*60*60))
	svc := NewWithClock(store, func() time.Time { return clock })
	ctx := context.Background()
	admin, err := svc.CreateUser(ctx, " Admin@Example.COM ", "Admin", domain.RoleAdmin, true)
	if err != nil {
		t.Fatal(err)
	}
	member, err := svc.CreateUser(ctx, "member@example.com", "Member", domain.RoleMember, true)
	if err != nil {
		t.Fatal(err)
	}
	other, err := svc.CreateUser(ctx, "other@example.com", "Other", domain.RoleMember, true)
	if err != nil {
		t.Fatal(err)
	}
	equipment, err := svc.CreateEquipment(ctx, admin.ID, CreateEquipmentInput{SerialNumber: " SN-42 ", Type: "Harness", Size: "M", Manufacturer: "Acme"})
	if err != nil {
		t.Fatal(err)
	}
	return fixture{svc: svc, database: database, admin: admin, member: member, other: other, equipment: equipment}
}

func (f fixture) close() { _ = f.database.Close() }

func TestEquipmentCRUDNormalizationAndHistoryPersistence(t *testing.T) {
	f := newFixture(t)
	defer f.close()
	ctx := context.Background()
	got, err := f.svc.GetEquipment(ctx, f.equipment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.SerialNumber != "sn-42" {
		t.Fatalf("serial = %q, want normalized value", got.SerialNumber)
	}
	if _, err := f.svc.CreateEquipment(ctx, f.admin.ID, CreateEquipmentInput{SerialNumber: "SN-42", Type: "Harness", Size: "M", Manufacturer: "Acme"}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("duplicate serial error = %v, want conflict", err)
	}
	history, err := f.svc.History(ctx, f.equipment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].EventType != domain.EventCreated {
		t.Fatalf("history = %#v, want one created event", history)
	}
	if _, err := f.svc.ChangeStatus(ctx, f.equipment.ID, domain.StatusLost, f.admin.ID, "not found"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.WriteOff(ctx, f.equipment.ID, f.admin.ID, "irrecoverable"); err != nil {
		t.Fatal(err)
	}
	visible, err := f.svc.ListEquipment(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(visible) != 0 {
		t.Fatalf("visible equipment = %d, want written-off equipment hidden", len(visible))
	}
	all, err := f.svc.ListEquipment(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all[0].Status != domain.StatusWrittenOff {
		t.Fatalf("all equipment = %#v", all)
	}
	history, err = f.svc.History(ctx, f.equipment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 3 {
		t.Fatalf("history length = %d, want 3", len(history))
	}
}

func TestIssueAndReturnAreAtomicAndPreserveHistory(t *testing.T) {
	f := newFixture(t)
	defer f.close()
	ctx := context.Background()
	issuance, err := f.svc.Issue(ctx, f.equipment.ID, f.member.ID, f.admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	item, _ := f.svc.GetEquipment(ctx, f.equipment.ID)
	if item.Status != domain.StatusOnStock {
		t.Fatalf("pending issue changed status to %s", item.Status)
	}
	if _, err := f.svc.ConfirmIssuance(ctx, issuance.ID, f.member.ID); err != nil {
		t.Fatal(err)
	}
	item, _ = f.svc.GetEquipment(ctx, f.equipment.ID)
	if item.Status != domain.StatusIssued {
		t.Fatalf("confirmed issue status = %s", item.Status)
	}
	ret, err := f.svc.StartReturn(ctx, issuance.ID, f.admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ConfirmReturn(ctx, ret.ID, f.member.ID); err != nil {
		t.Fatal(err)
	}
	item, _ = f.svc.GetEquipment(ctx, f.equipment.ID)
	if item.Status != domain.StatusOnStock {
		t.Fatalf("returned status = %s", item.Status)
	}
	closed, err := f.svc.History(ctx, f.equipment.ID)
	if err != nil {
		t.Fatal(err)
	}
	events := map[string]bool{}
	for _, event := range closed {
		events[event.EventType] = true
	}
	if len(closed) != 3 || !events[domain.EventCreated] || !events[domain.EventIssued] || !events[domain.EventReturned] {
		t.Fatalf("history = %#v", closed)
	}
	if _, err := f.svc.ConfirmReturn(ctx, ret.ID, f.member.ID); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("duplicate return = %v, want conflict", err)
	}
}

func TestMagicLinksAreSingleUseAndCreateSession(t *testing.T) {
	f := newFixture(t)
	defer f.close()
	ctx := context.Background()
	login, err := f.svc.CreateLoginLink(ctx, f.member.ID)
	if err != nil {
		t.Fatal(err)
	}
	session, err := f.svc.RedeemLoginLink(ctx, login.RawToken)
	if err != nil {
		t.Fatal(err)
	}
	if session.User.ID != f.member.ID || session.RawToken == "" {
		t.Fatalf("session = %#v", session)
	}
	if _, err := f.svc.RedeemLoginLink(ctx, login.RawToken); !errors.Is(err, domain.ErrUsed) {
		t.Fatalf("second redemption = %v, want used", err)
	}
	if _, err := f.svc.ValidateSession(ctx, session.RawToken); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.RevokeSession(ctx, session.RawToken); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ValidateSession(ctx, session.RawToken); !errors.Is(err, domain.ErrExpired) {
		t.Fatalf("revoked session = %v, want expired", err)
	}
}

func TestMagicLinkExpiresAfterTenMinutes(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "equipment.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	current := time.Date(2026, 8, 15, 10, 0, 0, 0, time.FixedZone("Europe/Berlin", 2*60*60))
	svc := NewWithClock(repository.NewStore(database), func() time.Time { return current })
	admin, err := svc.CreateUser(context.Background(), "expiry@example.com", "Expiry Admin", domain.RoleAdmin, true)
	if err != nil {
		t.Fatal(err)
	}
	link, err := svc.CreateLoginLink(context.Background(), admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	current = current.Add(11 * time.Minute)
	if _, err := svc.RedeemLoginLink(context.Background(), link.RawToken); !errors.Is(err, domain.ErrExpired) {
		t.Fatalf("expired link error = %v, want expired", err)
	}
}

func TestConfirmationMagicLinkUsesSameAtomicPath(t *testing.T) {
	f := newFixture(t)
	defer f.close()
	ctx := context.Background()
	issuance, err := f.svc.Issue(ctx, f.equipment.ID, f.member.ID, f.admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	link, err := f.svc.CreateIssuanceConfirmationLink(ctx, issuance.ID)
	if err != nil {
		t.Fatal(err)
	}
	event, err := f.svc.RedeemConfirmationLink(ctx, link.RawToken)
	if err != nil {
		t.Fatal(err)
	}
	if event != domain.EventIssued {
		t.Fatalf("event = %q", event)
	}
	if _, err := f.svc.RedeemConfirmationLink(ctx, link.RawToken); !errors.Is(err, domain.ErrUsed) {
		t.Fatalf("second confirmation = %v", err)
	}
}

func TestConcurrentIssueAllowsOnlyOneOpenIssuance(t *testing.T) {
	f := newFixture(t)
	defer f.close()
	ctx := context.Background()
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, memberID := range []string{f.member.ID, f.other.ID} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			_, err := f.svc.Issue(ctx, f.equipment.ID, id, f.admin.ID)
			results <- err
		}(memberID)
	}
	wg.Wait()
	close(results)
	var successes, conflicts int
	for err := range results {
		if err == nil {
			successes++
		} else if errors.Is(err, domain.ErrConflict) {
			conflicts++
		} else {
			t.Fatalf("unexpected concurrency error: %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d, want one each", successes, conflicts)
	}
}

func TestLossClosesPendingReturnAndCannotBeChangedToDamage(t *testing.T) {
	f := newFixture(t)
	defer f.close()
	ctx := context.Background()
	issuance, err := f.svc.Issue(ctx, f.equipment.ID, f.member.ID, f.admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ConfirmIssuance(ctx, issuance.ID, f.member.ID); err != nil {
		t.Fatal(err)
	}
	returnValue, err := f.svc.StartReturn(ctx, issuance.ID, f.admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ChangeStatus(ctx, f.equipment.ID, domain.StatusLost, f.admin.ID, "missing"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ConfirmReturn(ctx, returnValue.ID, f.member.ID); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("cancelled return confirmation = %v", err)
	}
	if _, err := f.svc.ChangeStatus(ctx, f.equipment.ID, domain.StatusDamaged, f.admin.ID, "found damaged"); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("lost to damaged = %v", err)
	}
}

func TestUserActivationRequiresAdminActor(t *testing.T) {
	f := newFixture(t)
	defer f.close()
	ctx := context.Background()
	if err := f.svc.SetUserActive(ctx, f.member.ID, f.other.ID, false); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("member activation error = %v", err)
	}
	if err := f.svc.SetUserActive(ctx, f.admin.ID, f.other.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.CreateLoginLink(ctx, f.other.ID); !errors.Is(err, domain.ErrInactive) {
		t.Fatalf("inactive login link = %v", err)
	}
}
