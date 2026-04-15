package repository

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/JekYUlll/Dipole/internal/model"
	platformCache "github.com/JekYUlll/Dipole/internal/platform/cache"
	"github.com/JekYUlll/Dipole/internal/store"
)

type ContactRepository struct{}

type cachedContactRelation struct {
	Exists  bool           `json:"exists"`
	Contact *model.Contact `json:"contact,omitempty"`
}

func NewContactRepository() *ContactRepository {
	return &ContactRepository{}
}

func (r *ContactRepository) AreFriends(userUUID, friendUUID string) (bool, error) {
	contact, err := r.GetContact(userUUID, friendUUID)
	if err != nil {
		return false, fmt.Errorf("check contacts friendship: %w", err)
	}

	return contact != nil, nil
}

func (r *ContactRepository) CanSendDirectMessage(userUUID, friendUUID string) (bool, error) {
	contact, err := r.GetContact(userUUID, friendUUID)
	if err != nil {
		return false, fmt.Errorf("get direct message outbound relation: %w", err)
	}
	if contact == nil || contact.Status != model.ContactStatusNormal {
		return false, nil
	}

	reverseContact, err := r.GetContact(friendUUID, userUUID)
	if err != nil {
		return false, fmt.Errorf("get direct message inbound relation: %w", err)
	}
	if reverseContact == nil || reverseContact.Status != model.ContactStatusNormal {
		return false, nil
	}

	return true, nil
}

func (r *ContactRepository) CreateFriendship(userOneUUID, userTwoUUID string) error {
	now := time.Now().UTC()
	contacts := []*model.Contact{
		{UserUUID: userOneUUID, FriendUUID: userTwoUUID, Status: model.ContactStatusNormal, CreatedAt: now, UpdatedAt: now},
		{UserUUID: userTwoUUID, FriendUUID: userOneUUID, Status: model.ContactStatusNormal, CreatedAt: now, UpdatedAt: now},
	}

	if err := store.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&contacts).Error; err != nil {
		return fmt.Errorf("create friendship: %w", err)
	}

	r.refreshContactRelationCache(contacts...)
	return nil
}

func (r *ContactRepository) DeleteFriendship(userOneUUID, userTwoUUID string) error {
	if err := store.DB.Where(
		"(user_uuid = ? AND friend_uuid = ?) OR (user_uuid = ? AND friend_uuid = ?)",
		userOneUUID,
		userTwoUUID,
		userTwoUUID,
		userOneUUID,
	).Delete(&model.Contact{}).Error; err != nil {
		return fmt.Errorf("delete friendship: %w", err)
	}

	r.invalidateContactRelationCache(userOneUUID, userTwoUUID)
	r.invalidateContactRelationCache(userTwoUUID, userOneUUID)
	return nil
}

func (r *ContactRepository) ListFriends(userUUID string) ([]*model.Contact, error) {
	var contacts []*model.Contact
	if err := store.DB.Where("user_uuid = ?", userUUID).Order("created_at DESC").Find(&contacts).Error; err != nil {
		return nil, fmt.Errorf("list contacts by user uuid: %w", err)
	}

	r.refreshContactRelationCache(contacts...)
	return contacts, nil
}

func (r *ContactRepository) GetContact(userUUID, friendUUID string) (*model.Contact, error) {
	if userUUID == "" || friendUUID == "" {
		return nil, nil
	}

	ctx, cancel := platformCache.NewContext()
	defer cancel()

	var cached cachedContactRelation
	if hit, err := platformCache.GetJSON(ctx, platformCache.ContactRelationKey(userUUID, friendUUID), &cached); err == nil && hit {
		if !cached.Exists {
			return nil, nil
		}
		return cached.Contact, nil
	}

	var contact model.Contact
	if err := store.DB.Where("user_uuid = ? AND friend_uuid = ?", userUUID, friendUUID).First(&contact).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			_ = platformCache.SetJSON(
				ctx,
				platformCache.ContactRelationKey(userUUID, friendUUID),
				cachedContactRelation{Exists: false},
				platformCache.ContactRelationTTL,
			)
			return nil, nil
		}

		return nil, fmt.Errorf("get contact: %w", err)
	}

	_ = platformCache.SetJSON(
		ctx,
		platformCache.ContactRelationKey(userUUID, friendUUID),
		cachedContactRelation{Exists: true, Contact: &contact},
		platformCache.ContactRelationTTL,
	)
	return &contact, nil
}

func (r *ContactRepository) UpdateContact(contact *model.Contact) error {
	if err := store.DB.Save(contact).Error; err != nil {
		return fmt.Errorf("update contact: %w", err)
	}

	r.refreshContactRelationCache(contact)
	return nil
}

func (r *ContactRepository) CreateApplication(application *model.ContactApplication) error {
	if err := store.DB.Create(application).Error; err != nil {
		return fmt.Errorf("create contact application: %w", err)
	}

	return nil
}

func (r *ContactRepository) GetApplicationByPair(applicantUUID, targetUUID string) (*model.ContactApplication, error) {
	var application model.ContactApplication
	if err := store.DB.Where("applicant_uuid = ? AND target_uuid = ?", applicantUUID, targetUUID).First(&application).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, fmt.Errorf("get contact application by pair: %w", err)
	}

	return &application, nil
}

func (r *ContactRepository) GetApplicationByID(id uint) (*model.ContactApplication, error) {
	var application model.ContactApplication
	if err := store.DB.First(&application, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, fmt.Errorf("get contact application by id: %w", err)
	}

	return &application, nil
}

func (r *ContactRepository) UpdateApplication(application *model.ContactApplication) error {
	if err := store.DB.Save(application).Error; err != nil {
		return fmt.Errorf("update contact application: %w", err)
	}

	return nil
}

func (r *ContactRepository) ListIncomingApplications(userUUID string) ([]*model.ContactApplication, error) {
	var applications []*model.ContactApplication
	if err := store.DB.Where("target_uuid = ?", userUUID).Order("created_at DESC").Find(&applications).Error; err != nil {
		return nil, fmt.Errorf("list incoming contact applications: %w", err)
	}

	return applications, nil
}

func (r *ContactRepository) ListOutgoingApplications(userUUID string) ([]*model.ContactApplication, error) {
	var applications []*model.ContactApplication
	if err := store.DB.Where("applicant_uuid = ?", userUUID).Order("created_at DESC").Find(&applications).Error; err != nil {
		return nil, fmt.Errorf("list outgoing contact applications: %w", err)
	}

	return applications, nil
}

func (r *ContactRepository) refreshContactRelationCache(contacts ...*model.Contact) {
	ctx, cancel := platformCache.NewContext()
	defer cancel()

	for _, contact := range contacts {
		if contact == nil {
			continue
		}
		_ = platformCache.SetJSON(
			ctx,
			platformCache.ContactRelationKey(contact.UserUUID, contact.FriendUUID),
			cachedContactRelation{Exists: true, Contact: contact},
			platformCache.ContactRelationTTL,
		)
	}
}

func (r *ContactRepository) invalidateContactRelationCache(userUUID, friendUUID string) {
	ctx, cancel := platformCache.NewContext()
	defer cancel()

	_ = platformCache.Delete(ctx, platformCache.ContactRelationKey(userUUID, friendUUID))
}
