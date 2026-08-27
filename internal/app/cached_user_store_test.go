package app

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/JekYUlll/Dipole/internal/model"
	platformBloom "github.com/JekYUlll/Dipole/internal/platform/bloom"
	"github.com/JekYUlll/Dipole/internal/store"
)

type userStoreSpy struct {
	users       map[string]*model.User
	getByUUIDs  int
	listByUUIDs int
}

func (s *userStoreSpy) Create(user *model.User) error {
	s.users[user.UUID] = cloneUser(user)
	return nil
}

func (s *userStoreSpy) UpsertAssistant(user *model.User) error {
	return s.Create(user)
}

func (s *userStoreSpy) GetByUUID(uuid string) (*model.User, error) {
	s.getByUUIDs++
	return cloneUser(s.users[uuid]), nil
}

func (s *userStoreSpy) GetByTelephone(telephone string) (*model.User, error) {
	for _, user := range s.users {
		if user.Telephone == telephone {
			return cloneUser(user), nil
		}
	}
	return nil, nil
}

func (s *userStoreSpy) Update(user *model.User) error {
	s.users[user.UUID] = cloneUser(user)
	return nil
}

func (s *userStoreSpy) SearchActive(string, string, int) ([]*model.User, error) {
	return nil, nil
}

func (s *userStoreSpy) List(string, *int8, int) ([]*model.User, error) {
	return nil, nil
}

func (s *userStoreSpy) ListByUUIDs(uuids []string) ([]*model.User, error) {
	s.listByUUIDs++
	users := make([]*model.User, 0, len(uuids))
	for _, uuid := range uuids {
		if user := s.users[uuid]; user != nil {
			users = append(users, cloneUser(user))
		}
	}
	return users, nil
}

func TestCachedUserStoreCachesReadsAndRefreshesUpdates(t *testing.T) {
	cleanup := setupCachedUserStoreTest(t)
	defer cleanup()

	backend := &userStoreSpy{users: map[string]*model.User{
		"U100": {UUID: "U100", Nickname: "Alice"},
	}}
	platformBloom.Load([]string{"U100"}, nil)
	users := NewCachedUserStore(backend)

	first, err := users.GetByUUID("U100")
	if err != nil || first == nil || first.Nickname != "Alice" {
		t.Fatalf("first read: user=%+v err=%v", first, err)
	}
	backend.users["U100"].Nickname = "database-change"
	second, err := users.GetByUUID("U100")
	if err != nil || second == nil || second.Nickname != "Alice" || backend.getByUUIDs != 1 {
		t.Fatalf("cached read: user=%+v calls=%d err=%v", second, backend.getByUUIDs, err)
	}

	second.Nickname = "Carol"
	if err := users.Update(second); err != nil {
		t.Fatalf("update: %v", err)
	}
	refreshed, err := users.GetByUUID("U100")
	if err != nil || refreshed == nil || refreshed.Nickname != "Carol" || backend.getByUUIDs != 1 {
		t.Fatalf("refreshed read: user=%+v calls=%d err=%v", refreshed, backend.getByUUIDs, err)
	}
}

func TestCachedUserStoreUsesBloomAndPreservesBatchOrder(t *testing.T) {
	cleanup := setupCachedUserStoreTest(t)
	defer cleanup()

	backend := &userStoreSpy{users: map[string]*model.User{
		"U100": {UUID: "U100"},
		"U200": {UUID: "U200"},
	}}
	platformBloom.Load([]string{"U100", "U200"}, nil)
	users := NewCachedUserStore(backend)

	missing, err := users.GetByUUID("U404")
	if err != nil || missing != nil || backend.getByUUIDs != 0 {
		t.Fatalf("bloom rejection: user=%+v calls=%d err=%v", missing, backend.getByUUIDs, err)
	}
	got, err := users.ListByUUIDs([]string{" U200 ", "U404", "U100", "U200", ""})
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	if len(got) != 2 || got[0].UUID != "U200" || got[1].UUID != "U100" || backend.listByUUIDs != 1 {
		t.Fatalf("ordered users=%+v calls=%d", got, backend.listByUUIDs)
	}
}

func setupCachedUserStoreTest(t *testing.T) func() {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("run miniredis: %v", err)
	}
	oldRDB := store.RDB
	store.RDB = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	platformBloom.Reset()
	return func() {
		_ = store.RDB.Close()
		store.RDB = oldRDB
		platformBloom.Reset()
		mr.Close()
	}
}

func cloneUser(user *model.User) *model.User {
	if user == nil {
		return nil
	}
	copy := *user
	return &copy
}
