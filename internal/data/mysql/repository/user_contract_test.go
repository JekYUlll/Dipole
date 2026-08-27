package repository_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/JekYUlll/Dipole/db/migrations"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/data/migration"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	sqlcRepository "github.com/JekYUlll/Dipole/internal/data/mysql/repository"
	"github.com/JekYUlll/Dipole/internal/model"
)

func TestUserRepositoryContract(t *testing.T) {
	db, _ := openContractDatabase(t)
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		t.Fatalf("create migration runner: %v", err)
	}
	if err := runner.Up(context.Background()); err != nil {
		t.Fatalf("migrate contract database: %v", err)
	}
	sqlcRepo, err := sqlcRepository.NewUserRepository(generated.New(db))
	if err != nil {
		t.Fatalf("create sqlc user repository: %v", err)
	}
	t.Run("sqlc", func(t *testing.T) {
		runUserContract(t, sqlcRepo, "sqlc")
	})
}

func runUserContract(t *testing.T, store application.UserStore, prefix string) {
	t.Helper()
	if err := store.Create(nil); err == nil {
		t.Fatal("expected nil user creation to fail")
	}
	if err := store.Update(nil); err == nil {
		t.Fatal("expected nil user update to fail")
	}

	telephoneBase := "1380000000"
	if prefix == "sqlc" {
		telephoneBase = "1390000000"
	}
	first := contractUser("U-"+prefix+"-1", telephoneBase+"1", "Alpha "+prefix, model.UserStatusNormal)
	second := contractUser("U-"+prefix+"-2", telephoneBase+"2", "Beta "+prefix, model.UserStatusNormal)
	disabled := contractUser("U-"+prefix+"-3", telephoneBase+"3", "Alpha disabled "+prefix, model.UserStatusDisabled)
	for _, user := range []*model.User{first, second, disabled} {
		if err := store.Create(user); err != nil {
			t.Fatalf("create %s: %v", user.UUID, err)
		}
		if user.ID == 0 || user.CreatedAt.IsZero() || user.UpdatedAt.IsZero() {
			t.Fatalf("expected generated fields for %s: %+v", user.UUID, user)
		}
	}

	byUUID, err := store.GetByUUID(" " + first.UUID + " ")
	if err != nil || byUUID == nil || byUUID.Telephone != first.Telephone {
		t.Fatalf("get by UUID: user=%+v err=%v", byUUID, err)
	}
	byTelephone, err := store.GetByTelephone(second.Telephone)
	if err != nil || byTelephone == nil || byTelephone.UUID != second.UUID {
		t.Fatalf("get by telephone: user=%+v err=%v", byTelephone, err)
	}
	missing, err := store.GetByUUID("U-missing-" + prefix)
	if err != nil || missing != nil {
		t.Fatalf("missing user: user=%+v err=%v", missing, err)
	}

	first.Nickname = "Updated " + prefix
	first.Email = prefix + "@example.com"
	first.Signature = "updated signature"
	if err := store.Update(first); err != nil {
		t.Fatalf("update user: %v", err)
	}
	if first.Nickname != "Updated "+prefix || first.Email != prefix+"@example.com" || first.UpdatedAt.IsZero() {
		t.Fatalf("unexpected updated user: %+v", first)
	}

	active, err := store.SearchActive("Alpha", second.UUID, 10)
	if err != nil {
		t.Fatalf("search active users: %v", err)
	}
	if len(active) != 0 {
		t.Fatalf("expected updated and disabled Alpha users to be excluded, got %+v", active)
	}
	active, err = store.SearchActive(prefix, second.UUID, 10)
	if err != nil || len(active) != 1 || active[0].UUID != first.UUID {
		all, listErr := store.List("", nil, 10)
		summary := make([]string, 0, len(all))
		for _, user := range all {
			summary = append(summary, fmt.Sprintf("%s:%s:%d", user.UUID, user.Nickname, user.Status))
		}
		t.Fatalf("search with exclusion: users=%+v err=%v all=%v list_err=%v", active, err, summary, listErr)
	}

	status := model.UserStatusDisabled
	listed, err := store.List(prefix, &status, 10)
	if err != nil || len(listed) != 1 || listed[0].UUID != disabled.UUID {
		t.Fatalf("list disabled users: users=%+v err=%v", listed, err)
	}
	ordered, err := store.ListByUUIDs([]string{second.UUID, " ", first.UUID, second.UUID, "U-missing"})
	if err != nil || len(ordered) != 2 || ordered[0].UUID != second.UUID || ordered[1].UUID != first.UUID {
		t.Fatalf("ordered batch: users=%+v err=%v", ordered, err)
	}

	assistant := contractUser("U-"+prefix+"-assistant", telephoneBase+"4", "Assistant old", model.UserStatusNormal)
	assistant.UserType = model.UserTypeAssistant
	assistant.Signature = "keep me"
	if err := store.UpsertAssistant(assistant); err != nil {
		t.Fatalf("insert assistant: %v", err)
	}
	assistant.Nickname = "Assistant new"
	assistant.Email = fmt.Sprintf("%s-assistant@example.com", prefix)
	assistant.Signature = "do not overwrite"
	if err := store.UpsertAssistant(assistant); err != nil {
		t.Fatalf("update assistant: %v", err)
	}
	storedAssistant, err := store.GetByUUID(assistant.UUID)
	if err != nil || storedAssistant == nil {
		t.Fatalf("get assistant: user=%+v err=%v", storedAssistant, err)
	}
	if storedAssistant.Nickname != "Assistant new" || storedAssistant.Signature != "keep me" {
		t.Fatalf("unexpected assistant update scope: %+v", storedAssistant)
	}
}

func contractUser(uuid, telephone, nickname string, status int8) *model.User {
	return &model.User{
		UUID:         uuid,
		Nickname:     nickname,
		Telephone:    telephone,
		Email:        "",
		Avatar:       model.DefaultAvatarURL,
		Signature:    "",
		PasswordHash: "hash",
		Status:       status,
	}
}
