package coremysql

import (
	"fmt"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	platformCache "github.com/JekYUlll/Dipole/internal/platform/cache"
)

var _ application.ContactStore = (*CachedContactStore)(nil)

type CachedContactStore struct {
	backend application.ContactStore
}

type cachedContactRelation struct {
	Exists  bool           `json:"exists"`
	Contact *model.Contact `json:"contact,omitempty"`
}

func NewCachedContactStore(backend application.ContactStore) *CachedContactStore {
	if backend == nil {
		panic("cached contact store backend is required")
	}
	return &CachedContactStore{backend: backend}
}

func (s *CachedContactStore) AreFriends(userUUID, friendUUID string) (bool, error) {
	contact, err := s.GetContact(userUUID, friendUUID)
	if err != nil {
		return false, fmt.Errorf("check contacts friendship: %w", err)
	}
	return contact != nil, nil
}

func (s *CachedContactStore) CanSendDirectMessage(userUUID, friendUUID string) (bool, error) {
	contact, err := s.GetContact(userUUID, friendUUID)
	if err != nil {
		return false, fmt.Errorf("get direct message outbound relation: %w", err)
	}
	if contact == nil || contact.Status != model.ContactStatusNormal {
		return false, nil
	}
	reverse, err := s.GetContact(friendUUID, userUUID)
	if err != nil {
		return false, fmt.Errorf("get direct message inbound relation: %w", err)
	}
	return reverse != nil && reverse.Status == model.ContactStatusNormal, nil
}

func (s *CachedContactStore) CreateFriendship(userOneUUID, userTwoUUID string) error {
	if err := s.backend.CreateFriendship(userOneUUID, userTwoUUID); err != nil {
		return err
	}
	s.invalidate(userOneUUID, userTwoUUID)
	s.invalidate(userTwoUUID, userOneUUID)
	return nil
}

func (s *CachedContactStore) DeleteFriendship(userOneUUID, userTwoUUID string) error {
	if err := s.backend.DeleteFriendship(userOneUUID, userTwoUUID); err != nil {
		return err
	}
	s.invalidate(userOneUUID, userTwoUUID)
	s.invalidate(userTwoUUID, userOneUUID)
	return nil
}

func (s *CachedContactStore) ListFriends(userUUID string) ([]*model.Contact, error) {
	contacts, err := s.backend.ListFriends(userUUID)
	if err != nil {
		return nil, err
	}
	for _, contact := range contacts {
		s.cache(contact)
	}
	return contacts, nil
}

func (s *CachedContactStore) GetContact(userUUID, friendUUID string) (*model.Contact, error) {
	if userUUID == "" || friendUUID == "" {
		return nil, nil
	}
	ctx, cancel := platformCache.NewContext()
	defer cancel()
	key := platformCache.ContactRelationKey(userUUID, friendUUID)
	var cached cachedContactRelation
	if hit, err := platformCache.GetJSON(ctx, key, &cached); err == nil && hit {
		if !cached.Exists {
			return nil, nil
		}
		return cached.Contact, nil
	}
	contact, err := s.backend.GetContact(userUUID, friendUUID)
	if err != nil {
		return nil, err
	}
	_ = platformCache.SetJSON(ctx, key, cachedContactRelation{
		Exists:  contact != nil,
		Contact: contact,
	}, platformCache.ContactRelationTTL)
	return contact, nil
}

func (s *CachedContactStore) UpdateContact(contact *model.Contact) error {
	if err := s.backend.UpdateContact(contact); err != nil {
		return err
	}
	s.cache(contact)
	return nil
}

func (s *CachedContactStore) CreateApplication(contactApplication *model.ContactApplication) error {
	return s.backend.CreateApplication(contactApplication)
}

func (s *CachedContactStore) GetApplicationByPair(applicantUUID, targetUUID string) (*model.ContactApplication, error) {
	return s.backend.GetApplicationByPair(applicantUUID, targetUUID)
}

func (s *CachedContactStore) GetApplicationByID(id uint) (*model.ContactApplication, error) {
	return s.backend.GetApplicationByID(id)
}

func (s *CachedContactStore) UpdateApplication(contactApplication *model.ContactApplication) error {
	return s.backend.UpdateApplication(contactApplication)
}

func (s *CachedContactStore) ListIncomingApplications(userUUID string) ([]*model.ContactApplication, error) {
	return s.backend.ListIncomingApplications(userUUID)
}

func (s *CachedContactStore) ListOutgoingApplications(userUUID string) ([]*model.ContactApplication, error) {
	return s.backend.ListOutgoingApplications(userUUID)
}

func (s *CachedContactStore) cache(contact *model.Contact) {
	if contact == nil {
		return
	}
	ctx, cancel := platformCache.NewContext()
	defer cancel()
	_ = platformCache.SetJSON(
		ctx,
		platformCache.ContactRelationKey(contact.UserUUID, contact.FriendUUID),
		cachedContactRelation{Exists: true, Contact: contact},
		platformCache.ContactRelationTTL,
	)
}

func (s *CachedContactStore) invalidate(userUUID, friendUUID string) {
	ctx, cancel := platformCache.NewContext()
	defer cancel()
	_ = platformCache.Delete(ctx, platformCache.ContactRelationKey(userUUID, friendUUID))
}
