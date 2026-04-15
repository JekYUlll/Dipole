package repository

import (
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/JekYUlll/Dipole/internal/model"
	platformBloom "github.com/JekYUlll/Dipole/internal/platform/bloom"
	"github.com/JekYUlll/Dipole/internal/store"
)

func TestUserRepositoryGetByUUIDUsesCacheAndUpdateRefreshes(t *testing.T) {
	cleanup := setupRepositoryCacheTest(t)
	defer cleanup()

	repo := NewUserRepository()
	user := &model.User{
		UUID:         "U100",
		Nickname:     "Alice",
		Telephone:    "13800000000",
		Avatar:       "avatar-a",
		PasswordHash: "hash",
		Status:       model.UserStatusNormal,
	}
	if err := store.DB.Create(user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	got, err := repo.GetByUUID("U100")
	if err != nil {
		t.Fatalf("get user first time: %v", err)
	}
	if got == nil || got.Nickname != "Alice" {
		t.Fatalf("unexpected user on first read: %+v", got)
	}

	if err := store.DB.Model(&model.User{}).Where("uuid = ?", "U100").Update("nickname", "Bob").Error; err != nil {
		t.Fatalf("update user directly in db: %v", err)
	}

	got, err = repo.GetByUUID("U100")
	if err != nil {
		t.Fatalf("get user second time: %v", err)
	}
	if got == nil || got.Nickname != "Alice" {
		t.Fatalf("expected cached nickname Alice, got %+v", got)
	}

	var refreshed model.User
	if err := store.DB.Where("uuid = ?", "U100").First(&refreshed).Error; err != nil {
		t.Fatalf("load updated user: %v", err)
	}
	refreshed.Nickname = "Carol"
	if err := repo.Update(&refreshed); err != nil {
		t.Fatalf("refresh cached user via repo update: %v", err)
	}

	got, err = repo.GetByUUID("U100")
	if err != nil {
		t.Fatalf("get user after repo update: %v", err)
	}
	if got == nil || got.Nickname != "Carol" {
		t.Fatalf("expected refreshed nickname Carol, got %+v", got)
	}
}

func TestGroupRepositoryGetByUUIDUsesCacheAndUpdateRefreshes(t *testing.T) {
	cleanup := setupRepositoryCacheTest(t)
	defer cleanup()

	repo := NewGroupRepository()
	group := &model.Group{
		UUID:        "G100",
		Name:        "Alpha",
		OwnerUUID:   "U100",
		MemberCount: 1,
		Status:      model.GroupStatusNormal,
	}
	if err := store.DB.Create(group).Error; err != nil {
		t.Fatalf("seed group: %v", err)
	}

	got, err := repo.GetByUUID("G100")
	if err != nil {
		t.Fatalf("get group first time: %v", err)
	}
	if got == nil || got.Name != "Alpha" {
		t.Fatalf("unexpected group on first read: %+v", got)
	}

	if err := store.DB.Model(&model.Group{}).Where("uuid = ?", "G100").Update("name", "Beta").Error; err != nil {
		t.Fatalf("update group directly in db: %v", err)
	}

	got, err = repo.GetByUUID("G100")
	if err != nil {
		t.Fatalf("get group second time: %v", err)
	}
	if got == nil || got.Name != "Alpha" {
		t.Fatalf("expected cached group name Alpha, got %+v", got)
	}

	var refreshed model.Group
	if err := store.DB.Where("uuid = ?", "G100").First(&refreshed).Error; err != nil {
		t.Fatalf("load updated group: %v", err)
	}
	refreshed.Name = "Gamma"
	if err := repo.Update(&refreshed); err != nil {
		t.Fatalf("refresh cached group via repo update: %v", err)
	}

	got, err = repo.GetByUUID("G100")
	if err != nil {
		t.Fatalf("get group after repo update: %v", err)
	}
	if got == nil || got.Name != "Gamma" {
		t.Fatalf("expected refreshed group name Gamma, got %+v", got)
	}
}

func TestGroupRepositoryListMembersUsesCacheAndInvalidatesOnAddMembers(t *testing.T) {
	cleanup := setupRepositoryCacheTest(t)
	defer cleanup()

	repo := NewGroupRepository()
	now := time.Now().UTC()
	group := &model.Group{
		UUID:        "G100",
		Name:        "Team",
		OwnerUUID:   "U100",
		MemberCount: 1,
		Status:      model.GroupStatusNormal,
	}
	memberA := &model.GroupMember{
		GroupUUID: "G100",
		UserUUID:  "U100",
		Role:      model.GroupMemberRoleOwner,
		JoinedAt:  now,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := store.DB.Create(group).Error; err != nil {
		t.Fatalf("seed group: %v", err)
	}
	if err := store.DB.Create(memberA).Error; err != nil {
		t.Fatalf("seed first member: %v", err)
	}

	members, err := repo.ListMembers("G100")
	if err != nil {
		t.Fatalf("list members first time: %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("expected 1 member on first read, got %d", len(members))
	}

	memberB := &model.GroupMember{
		GroupUUID: "G100",
		UserUUID:  "U200",
		Role:      model.GroupMemberRoleMember,
		JoinedAt:  now.Add(time.Second),
		CreatedAt: now.Add(time.Second),
		UpdatedAt: now.Add(time.Second),
	}
	if err := store.DB.Create(memberB).Error; err != nil {
		t.Fatalf("seed second member directly in db: %v", err)
	}

	members, err = repo.ListMembers("G100")
	if err != nil {
		t.Fatalf("list members second time: %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("expected cached 1 member before invalidation, got %d", len(members))
	}

	memberC := &model.GroupMember{
		GroupUUID: "G100",
		UserUUID:  "U300",
		Role:      model.GroupMemberRoleMember,
		JoinedAt:  now.Add(2 * time.Second),
		CreatedAt: now.Add(2 * time.Second),
		UpdatedAt: now.Add(2 * time.Second),
	}
	if err := repo.AddMembers("G100", []*model.GroupMember{memberC}); err != nil {
		t.Fatalf("add members via repository: %v", err)
	}

	members, err = repo.ListMembers("G100")
	if err != nil {
		t.Fatalf("list members after invalidation: %v", err)
	}
	if len(members) != 3 {
		t.Fatalf("expected 3 members after invalidation and reload, got %d", len(members))
	}
}

func TestContactRepositoryGetContactUsesCacheAndUpdateRefreshes(t *testing.T) {
	cleanup := setupRepositoryCacheTest(t)
	defer cleanup()

	repo := NewContactRepository()
	contact := &model.Contact{
		UserUUID:   "U100",
		FriendUUID: "U200",
		Remark:     "alpha",
		Status:     model.ContactStatusNormal,
	}
	if err := store.DB.Create(contact).Error; err != nil {
		t.Fatalf("seed contact: %v", err)
	}

	got, err := repo.GetContact("U100", "U200")
	if err != nil {
		t.Fatalf("get contact first time: %v", err)
	}
	if got == nil || got.Remark != "alpha" {
		t.Fatalf("unexpected contact on first read: %+v", got)
	}

	if err := store.DB.Model(&model.Contact{}).
		Where("user_uuid = ? AND friend_uuid = ?", "U100", "U200").
		Update("remark", "beta").Error; err != nil {
		t.Fatalf("update contact directly in db: %v", err)
	}

	got, err = repo.GetContact("U100", "U200")
	if err != nil {
		t.Fatalf("get contact second time: %v", err)
	}
	if got == nil || got.Remark != "alpha" {
		t.Fatalf("expected cached remark alpha, got %+v", got)
	}

	var refreshed model.Contact
	if err := store.DB.Where("user_uuid = ? AND friend_uuid = ?", "U100", "U200").First(&refreshed).Error; err != nil {
		t.Fatalf("load updated contact: %v", err)
	}
	refreshed.Remark = "gamma"
	if err := repo.UpdateContact(&refreshed); err != nil {
		t.Fatalf("refresh cached contact via repo update: %v", err)
	}

	got, err = repo.GetContact("U100", "U200")
	if err != nil {
		t.Fatalf("get contact after repo update: %v", err)
	}
	if got == nil || got.Remark != "gamma" {
		t.Fatalf("expected refreshed remark gamma, got %+v", got)
	}
}

func TestContactRepositoryCanSendDirectMessageUsesCacheAndDeleteInvalidates(t *testing.T) {
	cleanup := setupRepositoryCacheTest(t)
	defer cleanup()

	repo := NewContactRepository()
	now := time.Now().UTC()
	contacts := []*model.Contact{
		{
			UserUUID:   "U100",
			FriendUUID: "U200",
			Status:     model.ContactStatusNormal,
			CreatedAt:  now,
			UpdatedAt:  now,
		},
		{
			UserUUID:   "U200",
			FriendUUID: "U100",
			Status:     model.ContactStatusNormal,
			CreatedAt:  now,
			UpdatedAt:  now,
		},
	}
	if err := store.DB.Create(&contacts).Error; err != nil {
		t.Fatalf("seed bidirectional contacts: %v", err)
	}

	allowed, err := repo.CanSendDirectMessage("U100", "U200")
	if err != nil {
		t.Fatalf("check direct permission first time: %v", err)
	}
	if !allowed {
		t.Fatalf("expected direct message permission to be allowed")
	}

	if err := repo.DeleteFriendship("U100", "U200"); err != nil {
		t.Fatalf("delete friendship: %v", err)
	}

	allowed, err = repo.CanSendDirectMessage("U100", "U200")
	if err != nil {
		t.Fatalf("check direct permission after delete: %v", err)
	}
	if allowed {
		t.Fatalf("expected direct message permission to be denied after delete")
	}
}

func TestUserRepositoryGetByUUIDSkipsDBWhenBloomRejects(t *testing.T) {
	cleanup := setupRepositoryCacheTest(t)
	defer cleanup()

	repo := NewUserRepository()
	platformBloom.Load([]string{"U100"}, nil)

	if err := store.DB.Create(&model.User{
		UUID:         "U100",
		Nickname:     "Alice",
		Telephone:    "13800000000",
		Avatar:       "avatar-a",
		PasswordHash: "hash",
		Status:       model.UserStatusNormal,
	}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	got, err := repo.GetByUUID("U404")
	if err != nil {
		t.Fatalf("get missing user with bloom reject: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil user when bloom rejects, got %+v", got)
	}
}

func TestGroupRepositoryGetByUUIDSkipsDBWhenBloomRejects(t *testing.T) {
	cleanup := setupRepositoryCacheTest(t)
	defer cleanup()

	repo := NewGroupRepository()
	platformBloom.Load(nil, []string{"G100"})

	if err := store.DB.Create(&model.Group{
		UUID:        "G100",
		Name:        "Alpha",
		OwnerUUID:   "U100",
		MemberCount: 1,
		Status:      model.GroupStatusNormal,
	}).Error; err != nil {
		t.Fatalf("seed group: %v", err)
	}

	got, err := repo.GetByUUID("G404")
	if err != nil {
		t.Fatalf("get missing group with bloom reject: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil group when bloom rejects, got %+v", got)
	}
}

func setupRepositoryCacheTest(t *testing.T) func() {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Group{}, &model.GroupMember{}, &model.Contact{}); err != nil {
		t.Fatalf("auto migrate sqlite: %v", err)
	}

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("run miniredis: %v", err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	oldDB := store.DB
	oldRDB := store.RDB
	store.DB = db
	store.RDB = rdb
	platformBloom.Reset()

	return func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		store.DB = oldDB
		store.RDB = oldRDB
		platformBloom.Reset()
		_ = rdb.Close()
		mr.Close()
	}
}
