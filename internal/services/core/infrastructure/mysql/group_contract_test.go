package coremysql_test

import (
	"context"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/data/migration"
	mysqlData "github.com/JekYUlll/Dipole/internal/data/mysql"
	"github.com/JekYUlll/Dipole/internal/model"
	sqlcRepository "github.com/JekYUlll/Dipole/internal/services/core/infrastructure/mysql"
)

func TestGroupRepositoryContract(t *testing.T) {
	db, _ := openContractDatabase(t)
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		t.Fatalf("create migration runner: %v", err)
	}
	if err := runner.Up(context.Background()); err != nil {
		t.Fatalf("migrate contract database: %v", err)
	}
	mysqlStore, err := mysqlData.NewStore(db)
	if err != nil {
		t.Fatalf("create MySQL store: %v", err)
	}
	sqlcRepo, err := sqlcRepository.NewGroupRepository(mysqlStore)
	if err != nil {
		t.Fatalf("create sqlc group repository: %v", err)
	}
	t.Run("sqlc", func(t *testing.T) {
		runGroupContract(t, sqlcRepo, "sqlc")
	})
}

func runGroupContract(t *testing.T, store application.GroupStore, prefix string) {
	t.Helper()
	if err := store.Create(nil, nil); err == nil {
		t.Fatal("expected nil group creation to fail")
	}
	if err := store.Update(nil); err == nil {
		t.Fatal("expected nil group update to fail")
	}
	if err := store.AddMembers("G-empty", nil); err != nil {
		t.Fatalf("empty member addition: %v", err)
	}
	if err := store.RemoveMembers("G-empty", nil); err != nil {
		t.Fatalf("empty member removal: %v", err)
	}
	emptyGroup, err := store.GetByUUID(" ")
	if err != nil || emptyGroup != nil {
		t.Fatalf("empty group lookup: group=%+v err=%v", emptyGroup, err)
	}
	emptyMembers, err := store.ListMembers(" ")
	if err != nil || len(emptyMembers) != 0 {
		t.Fatalf("empty member list: members=%+v err=%v", emptyMembers, err)
	}

	now := time.Now().UTC().Truncate(time.Millisecond)
	rollbackUUID := "G-" + prefix + "-rollback"
	rollbackGroup := contractGroup(rollbackUUID, "U-owner-"+prefix, 2, now)
	duplicateMembers := []*model.GroupMember{
		contractGroupMember(rollbackUUID, "U-duplicate-"+prefix, model.GroupMemberRoleOwner, now),
		contractGroupMember(rollbackUUID, "U-duplicate-"+prefix, model.GroupMemberRoleMember, now.Add(time.Second)),
	}
	if err := store.Create(rollbackGroup, duplicateMembers); err == nil {
		t.Fatal("expected duplicate initial members to roll back group creation")
	}
	rolledBack, err := store.GetByUUID(rollbackUUID)
	if err != nil || rolledBack != nil {
		t.Fatalf("group transaction rollback: group=%+v err=%v", rolledBack, err)
	}

	groupUUID := "G-" + prefix + "-main"
	ownerUUID := "U-" + prefix + "-owner"
	memberUUID := "U-" + prefix + "-member"
	group := contractGroup(groupUUID, ownerUUID, 2, now)
	owner := contractGroupMember(groupUUID, ownerUUID, model.GroupMemberRoleOwner, now)
	member := contractGroupMember(groupUUID, memberUUID, model.GroupMemberRoleMember, now.Add(time.Second))
	if err := store.Create(group, []*model.GroupMember{owner, nil, member}); err != nil {
		t.Fatalf("create group: %v", err)
	}
	if group.ID == 0 || group.CreatedAt.IsZero() || group.UpdatedAt.IsZero() {
		t.Fatalf("expected generated group fields: %+v", group)
	}
	loaded, err := store.GetByUUID(" " + groupUUID + " ")
	if err != nil || loaded == nil || loaded.MemberCount != 2 {
		t.Fatalf("get group: group=%+v err=%v", loaded, err)
	}
	loadedOwner, err := store.GetMember(groupUUID, ownerUUID)
	if err != nil || loadedOwner == nil || loadedOwner.Role != model.GroupMemberRoleOwner {
		t.Fatalf("get owner member: member=%+v err=%v", loadedOwner, err)
	}
	members, err := store.ListMembers(groupUUID)
	if err != nil || len(members) != 2 || members[0].UserUUID != ownerUUID || members[1].UserUUID != memberUUID {
		t.Fatalf("ordered initial members: members=%+v err=%v", members, err)
	}

	newMemberUUID := "U-" + prefix + "-new"
	newMember := contractGroupMember(groupUUID, newMemberUUID, model.GroupMemberRoleMember, now.Add(2*time.Second))
	if err := store.AddMembers(groupUUID, []*model.GroupMember{nil, member, newMember, newMember}); err != nil {
		t.Fatalf("add members: %v", err)
	}
	afterAdd, err := store.GetByUUID(groupUUID)
	if err != nil || afterAdd == nil || afterAdd.MemberCount != 3 {
		t.Fatalf("member count after duplicate add: group=%+v err=%v", afterAdd, err)
	}
	if err := store.AddMembers(groupUUID, []*model.GroupMember{newMember}); err != nil {
		t.Fatalf("repeat member add: %v", err)
	}
	afterRepeat, err := store.GetByUUID(groupUUID)
	if err != nil || afterRepeat == nil || afterRepeat.MemberCount != 3 {
		t.Fatalf("member count after repeated add: group=%+v err=%v", afterRepeat, err)
	}

	afterRepeat.Name = "Updated " + prefix
	afterRepeat.Notice = "updated notice"
	if err := store.Update(afterRepeat); err != nil {
		t.Fatalf("update group: %v", err)
	}
	if afterRepeat.Name != "Updated "+prefix || afterRepeat.MemberCount != 3 || afterRepeat.UpdatedAt.IsZero() {
		t.Fatalf("unexpected updated group: %+v", afterRepeat)
	}

	if err := store.RemoveMembers(groupUUID, []string{newMemberUUID, "U-missing", newMemberUUID}); err != nil {
		t.Fatalf("remove members: %v", err)
	}
	afterBatchRemove, err := store.GetByUUID(groupUUID)
	if err != nil || afterBatchRemove == nil || afterBatchRemove.MemberCount != 2 {
		t.Fatalf("member count after batch remove: group=%+v err=%v", afterBatchRemove, err)
	}
	if err := store.RemoveMember(groupUUID, memberUUID); err != nil {
		t.Fatalf("remove member: %v", err)
	}
	if err := store.RemoveMember(groupUUID, memberUUID); err != nil {
		t.Fatalf("repeat member removal: %v", err)
	}
	afterSingleRemove, err := store.GetByUUID(groupUUID)
	if err != nil || afterSingleRemove == nil || afterSingleRemove.MemberCount != 1 {
		t.Fatalf("member count after repeated remove: group=%+v err=%v", afterSingleRemove, err)
	}
	remaining, err := store.ListMembers(groupUUID)
	if err != nil || len(remaining) != 1 || remaining[0].UserUUID != ownerUUID {
		t.Fatalf("remaining members: members=%+v err=%v", remaining, err)
	}
}

func contractGroup(uuid, ownerUUID string, memberCount int, now time.Time) *model.Group {
	return &model.Group{
		UUID:        uuid,
		Name:        "Group " + uuid,
		Notice:      "",
		Avatar:      "",
		OwnerUUID:   ownerUUID,
		MemberCount: memberCount,
		Status:      model.GroupStatusNormal,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func contractGroupMember(groupUUID, userUUID string, role int8, joinedAt time.Time) *model.GroupMember {
	return &model.GroupMember{
		GroupUUID: groupUUID,
		UserUUID:  userUUID,
		Role:      role,
		JoinedAt:  joinedAt,
		CreatedAt: joinedAt,
		UpdatedAt: joinedAt,
	}
}
