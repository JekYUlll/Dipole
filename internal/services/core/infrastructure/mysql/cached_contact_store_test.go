package coremysql

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/platform/cache"
)

type contactStoreSpy struct {
	contacts map[string]*model.Contact
	getCalls int
}

func (s *contactStoreSpy) AreFriends(userUUID, friendUUID string) (bool, error) {
	contact, err := s.GetContact(userUUID, friendUUID)
	return contact != nil, err
}

func (s *contactStoreSpy) CanSendDirectMessage(userUUID, friendUUID string) (bool, error) {
	left, _ := s.GetContact(userUUID, friendUUID)
	right, _ := s.GetContact(friendUUID, userUUID)
	return left != nil && right != nil, nil
}

func (s *contactStoreSpy) CreateFriendship(userOneUUID, userTwoUUID string) error {
	s.contacts[contactSpyKey(userOneUUID, userTwoUUID)] = &model.Contact{UserUUID: userOneUUID, FriendUUID: userTwoUUID}
	s.contacts[contactSpyKey(userTwoUUID, userOneUUID)] = &model.Contact{UserUUID: userTwoUUID, FriendUUID: userOneUUID}
	return nil
}

func (s *contactStoreSpy) DeleteFriendship(userOneUUID, userTwoUUID string) error {
	delete(s.contacts, contactSpyKey(userOneUUID, userTwoUUID))
	delete(s.contacts, contactSpyKey(userTwoUUID, userOneUUID))
	return nil
}

func (s *contactStoreSpy) ListFriends(userUUID string) ([]*model.Contact, error) {
	contacts := make([]*model.Contact, 0)
	for _, contact := range s.contacts {
		if contact.UserUUID == userUUID {
			contacts = append(contacts, cloneContact(contact))
		}
	}
	return contacts, nil
}

func (s *contactStoreSpy) GetContact(userUUID, friendUUID string) (*model.Contact, error) {
	s.getCalls++
	return cloneContact(s.contacts[contactSpyKey(userUUID, friendUUID)]), nil
}

func (s *contactStoreSpy) UpdateContact(contact *model.Contact) error {
	s.contacts[contactSpyKey(contact.UserUUID, contact.FriendUUID)] = cloneContact(contact)
	return nil
}

func (s *contactStoreSpy) CreateApplication(*model.ContactApplication) error { return nil }
func (s *contactStoreSpy) GetApplicationByPair(string, string) (*model.ContactApplication, error) {
	return nil, nil
}
func (s *contactStoreSpy) GetApplicationByID(uint) (*model.ContactApplication, error) {
	return nil, nil
}
func (s *contactStoreSpy) UpdateApplication(*model.ContactApplication) error { return nil }
func (s *contactStoreSpy) ListIncomingApplications(string) ([]*model.ContactApplication, error) {
	return nil, nil
}
func (s *contactStoreSpy) ListOutgoingApplications(string) ([]*model.ContactApplication, error) {
	return nil, nil
}

func TestCachedContactStoreCachesMissesAndRefreshesUpdates(t *testing.T) {
	cleanup := setupCachedContactStoreTest(t)
	defer cleanup()

	backend := &contactStoreSpy{contacts: map[string]*model.Contact{
		contactSpyKey("U100", "U200"): {UserUUID: "U100", FriendUUID: "U200", Remark: "old"},
	}}
	contacts := NewCachedContactStore(backend)

	first, err := contacts.GetContact("U100", "U200")
	if err != nil || first == nil || first.Remark != "old" {
		t.Fatalf("first read: contact=%+v err=%v", first, err)
	}
	backend.contacts[contactSpyKey("U100", "U200")].Remark = "database-change"
	second, err := contacts.GetContact("U100", "U200")
	if err != nil || second == nil || second.Remark != "old" || backend.getCalls != 1 {
		t.Fatalf("cached read: contact=%+v calls=%d err=%v", second, backend.getCalls, err)
	}

	second.Remark = "new"
	if err := contacts.UpdateContact(second); err != nil {
		t.Fatalf("update contact: %v", err)
	}
	updated, err := contacts.GetContact("U100", "U200")
	if err != nil || updated == nil || updated.Remark != "new" || backend.getCalls != 1 {
		t.Fatalf("updated read: contact=%+v calls=%d err=%v", updated, backend.getCalls, err)
	}

	missing, err := contacts.GetContact("U100", "U404")
	if err != nil || missing != nil {
		t.Fatalf("first missing read: contact=%+v err=%v", missing, err)
	}
	missing, err = contacts.GetContact("U100", "U404")
	if err != nil || missing != nil || backend.getCalls != 2 {
		t.Fatalf("cached missing read: contact=%+v calls=%d err=%v", missing, backend.getCalls, err)
	}
}

func TestCachedContactStoreInvalidatesBothDirections(t *testing.T) {
	cleanup := setupCachedContactStoreTest(t)
	defer cleanup()

	backend := &contactStoreSpy{contacts: map[string]*model.Contact{
		contactSpyKey("U100", "U200"): {UserUUID: "U100", FriendUUID: "U200"},
		contactSpyKey("U200", "U100"): {UserUUID: "U200", FriendUUID: "U100"},
	}}
	contacts := NewCachedContactStore(backend)
	allowed, err := contacts.CanSendDirectMessage("U100", "U200")
	if err != nil || !allowed {
		t.Fatalf("initial permission: allowed=%v err=%v", allowed, err)
	}
	if err := contacts.DeleteFriendship("U100", "U200"); err != nil {
		t.Fatalf("delete friendship: %v", err)
	}
	allowed, err = contacts.CanSendDirectMessage("U100", "U200")
	if err != nil || allowed {
		t.Fatalf("permission after delete: allowed=%v err=%v", allowed, err)
	}
}

func setupCachedContactStoreTest(t *testing.T) func() {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("run miniredis: %v", err)
	}
	oldRDB := cache.RDB
	cache.RDB = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return func() {
		_ = cache.RDB.Close()
		cache.RDB = oldRDB
		mr.Close()
	}
}

func contactSpyKey(userUUID, friendUUID string) string {
	return userUUID + ":" + friendUUID
}

func cloneContact(contact *model.Contact) *model.Contact {
	if contact == nil {
		return nil
	}
	copy := *contact
	return &copy
}
