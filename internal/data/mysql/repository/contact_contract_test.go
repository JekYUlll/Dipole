package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/data/migration"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	sqlcRepository "github.com/JekYUlll/Dipole/internal/data/mysql/repository"
	"github.com/JekYUlll/Dipole/internal/model"
	gormRepository "github.com/JekYUlll/Dipole/internal/repository"
	gormMySQL "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestContactRepositoryContract(t *testing.T) {
	db, dsn := openContractDatabase(t)
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		t.Fatalf("create migration runner: %v", err)
	}
	if err := runner.Up(context.Background()); err != nil {
		t.Fatalf("migrate contract database: %v", err)
	}
	gormDB, err := gorm.Open(gormMySQL.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open GORM contract database: %v", err)
	}
	sqlcRepo, err := sqlcRepository.NewContactRepository(generated.New(db))
	if err != nil {
		t.Fatalf("create sqlc contact repository: %v", err)
	}
	stores := map[string]application.ContactStore{
		"gorm": gormRepository.NewContactRepositoryWithDB(gormDB),
		"sqlc": sqlcRepo,
	}
	for name, store := range stores {
		t.Run(name, func(t *testing.T) {
			runContactContract(t, store, name)
		})
	}
}

func runContactContract(t *testing.T, store application.ContactStore, prefix string) {
	t.Helper()
	if err := store.UpdateContact(nil); err == nil {
		t.Fatal("expected nil contact update to fail")
	}
	if err := store.CreateApplication(nil); err == nil {
		t.Fatal("expected nil application creation to fail")
	}
	if err := store.UpdateApplication(nil); err == nil {
		t.Fatal("expected nil application update to fail")
	}
	empty, err := store.GetContact("", "U-any")
	if err != nil || empty != nil {
		t.Fatalf("empty contact lookup: contact=%+v err=%v", empty, err)
	}

	userOne := "U-" + prefix + "-contact-1"
	userTwo := "U-" + prefix + "-contact-2"
	if err := store.CreateFriendship(userOne, userTwo); err != nil {
		t.Fatalf("create friendship: %v", err)
	}
	friends, err := store.AreFriends(userOne, userTwo)
	if err != nil || !friends {
		t.Fatalf("are friends: friends=%v err=%v", friends, err)
	}
	allowed, err := store.CanSendDirectMessage(userOne, userTwo)
	if err != nil || !allowed {
		t.Fatalf("direct permission: allowed=%v err=%v", allowed, err)
	}

	contact, err := store.GetContact(userOne, userTwo)
	if err != nil || contact == nil || contact.ID == 0 || contact.CreatedAt.IsZero() {
		t.Fatalf("get contact: contact=%+v err=%v", contact, err)
	}
	contact.Remark = "project partner"
	contact.Status = model.ContactStatusBlocked
	if err := store.UpdateContact(contact); err != nil {
		t.Fatalf("update contact: %v", err)
	}
	if contact.Remark != "project partner" || contact.Status != model.ContactStatusBlocked || contact.UpdatedAt.IsZero() {
		t.Fatalf("unexpected updated contact: %+v", contact)
	}
	allowed, err = store.CanSendDirectMessage(userOne, userTwo)
	if err != nil || allowed {
		t.Fatalf("blocked direct permission: allowed=%v err=%v", allowed, err)
	}

	if err := store.CreateFriendship(userOne, userTwo); err != nil {
		t.Fatalf("repeat friendship creation: %v", err)
	}
	preserved, err := store.GetContact(userOne, userTwo)
	if err != nil || preserved == nil || preserved.Status != model.ContactStatusBlocked || preserved.Remark != "project partner" {
		t.Fatalf("repeat creation overwrote contact: contact=%+v err=%v", preserved, err)
	}
	listed, err := store.ListFriends(userOne)
	if err != nil || len(listed) != 1 || listed[0].FriendUUID != userTwo {
		t.Fatalf("list friends: contacts=%+v err=%v", listed, err)
	}

	expiresAt := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Millisecond)
	contactApplication := &model.ContactApplication{
		ApplicantUUID: userOne,
		TargetUUID:    userTwo,
		Message:       "hello",
		Status:        model.ContactApplicationPending,
		ExpiresAt:     &expiresAt,
	}
	if err := store.CreateApplication(contactApplication); err != nil {
		t.Fatalf("create application: %v", err)
	}
	if contactApplication.ID == 0 || contactApplication.CreatedAt.IsZero() || contactApplication.UpdatedAt.IsZero() {
		t.Fatalf("expected generated application fields: %+v", contactApplication)
	}
	byPair, err := store.GetApplicationByPair(userOne, userTwo)
	if err != nil || byPair == nil || byPair.ID != contactApplication.ID || byPair.ExpiresAt == nil {
		t.Fatalf("get application by pair: application=%+v err=%v", byPair, err)
	}
	byID, err := store.GetApplicationByID(contactApplication.ID)
	if err != nil || byID == nil || byID.Message != "hello" {
		t.Fatalf("get application by ID: application=%+v err=%v", byID, err)
	}

	handledAt := time.Now().UTC().Truncate(time.Millisecond)
	contactApplication.Status = model.ContactApplicationAccepted
	contactApplication.Message = "accepted"
	contactApplication.HandledAt = &handledAt
	if err := store.UpdateApplication(contactApplication); err != nil {
		t.Fatalf("update application: %v", err)
	}
	if contactApplication.Status != model.ContactApplicationAccepted || contactApplication.HandledAt == nil || contactApplication.Message != "accepted" {
		t.Fatalf("unexpected updated application: %+v", contactApplication)
	}
	incoming, err := store.ListIncomingApplications(userTwo)
	if err != nil || len(incoming) != 1 || incoming[0].ID != contactApplication.ID {
		t.Fatalf("list incoming applications: applications=%+v err=%v", incoming, err)
	}
	outgoing, err := store.ListOutgoingApplications(userOne)
	if err != nil || len(outgoing) != 1 || outgoing[0].ID != contactApplication.ID {
		t.Fatalf("list outgoing applications: applications=%+v err=%v", outgoing, err)
	}
	missingPair, err := store.GetApplicationByPair(userTwo, userOne)
	if err != nil || missingPair != nil {
		t.Fatalf("missing application pair: application=%+v err=%v", missingPair, err)
	}
	missingID, err := store.GetApplicationByID(contactApplication.ID + 100000)
	if err != nil || missingID != nil {
		t.Fatalf("missing application ID: application=%+v err=%v", missingID, err)
	}

	if err := store.DeleteFriendship(userOne, userTwo); err != nil {
		t.Fatalf("delete friendship: %v", err)
	}
	left, err := store.GetContact(userOne, userTwo)
	if err != nil || left != nil {
		t.Fatalf("left contact after delete: contact=%+v err=%v", left, err)
	}
	right, err := store.GetContact(userTwo, userOne)
	if err != nil || right != nil {
		t.Fatalf("right contact after delete: contact=%+v err=%v", right, err)
	}
}
