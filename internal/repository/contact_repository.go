package repository

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/store"
)

var _ application.ContactStore = (*ContactRepository)(nil)

type ContactRepository struct {
	db *gorm.DB
}

func NewContactRepository() *ContactRepository {
	return &ContactRepository{}
}

func NewContactRepositoryWithDB(db *gorm.DB) *ContactRepository {
	return &ContactRepository{db: db}
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
	reverse, err := r.GetContact(friendUUID, userUUID)
	if err != nil {
		return false, fmt.Errorf("get direct message inbound relation: %w", err)
	}
	return reverse != nil && reverse.Status == model.ContactStatusNormal, nil
}

func (r *ContactRepository) CreateFriendship(userOneUUID, userTwoUUID string) error {
	now := time.Now().UTC()
	contacts := []*model.Contact{
		{UserUUID: userOneUUID, FriendUUID: userTwoUUID, Status: model.ContactStatusNormal, CreatedAt: now, UpdatedAt: now},
		{UserUUID: userTwoUUID, FriendUUID: userOneUUID, Status: model.ContactStatusNormal, CreatedAt: now, UpdatedAt: now},
	}
	if err := r.database().Clauses(clause.OnConflict{DoNothing: true}).Create(&contacts).Error; err != nil {
		return fmt.Errorf("create friendship: %w", err)
	}
	return nil
}

func (r *ContactRepository) DeleteFriendship(userOneUUID, userTwoUUID string) error {
	if err := r.database().Where(
		"(user_uuid = ? AND friend_uuid = ?) OR (user_uuid = ? AND friend_uuid = ?)",
		userOneUUID,
		userTwoUUID,
		userTwoUUID,
		userOneUUID,
	).Delete(&model.Contact{}).Error; err != nil {
		return fmt.Errorf("delete friendship: %w", err)
	}
	return nil
}

func (r *ContactRepository) ListFriends(userUUID string) ([]*model.Contact, error) {
	var contacts []*model.Contact
	if err := r.database().Where("user_uuid = ?", userUUID).Order("created_at DESC").Find(&contacts).Error; err != nil {
		return nil, fmt.Errorf("list contacts by user uuid: %w", err)
	}
	return contacts, nil
}

func (r *ContactRepository) GetContact(userUUID, friendUUID string) (*model.Contact, error) {
	if userUUID == "" || friendUUID == "" {
		return nil, nil
	}
	var contact model.Contact
	if err := r.database().Where("user_uuid = ? AND friend_uuid = ?", userUUID, friendUUID).First(&contact).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get contact: %w", err)
	}
	return &contact, nil
}

func (r *ContactRepository) UpdateContact(contact *model.Contact) error {
	if err := r.database().Save(contact).Error; err != nil {
		return fmt.Errorf("update contact: %w", err)
	}
	return nil
}

func (r *ContactRepository) CreateApplication(contactApplication *model.ContactApplication) error {
	if err := r.database().Create(contactApplication).Error; err != nil {
		return fmt.Errorf("create contact application: %w", err)
	}
	return nil
}

func (r *ContactRepository) GetApplicationByPair(applicantUUID, targetUUID string) (*model.ContactApplication, error) {
	var contactApplication model.ContactApplication
	if err := r.database().Where("applicant_uuid = ? AND target_uuid = ?", applicantUUID, targetUUID).First(&contactApplication).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get contact application by pair: %w", err)
	}
	return &contactApplication, nil
}

func (r *ContactRepository) GetApplicationByID(id uint) (*model.ContactApplication, error) {
	var contactApplication model.ContactApplication
	if err := r.database().First(&contactApplication, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get contact application by id: %w", err)
	}
	return &contactApplication, nil
}

func (r *ContactRepository) UpdateApplication(contactApplication *model.ContactApplication) error {
	if err := r.database().Save(contactApplication).Error; err != nil {
		return fmt.Errorf("update contact application: %w", err)
	}
	return nil
}

func (r *ContactRepository) ListIncomingApplications(userUUID string) ([]*model.ContactApplication, error) {
	var contactApplications []*model.ContactApplication
	if err := r.database().Where("target_uuid = ?", userUUID).Order("created_at DESC").Find(&contactApplications).Error; err != nil {
		return nil, fmt.Errorf("list incoming contact applications: %w", err)
	}
	return contactApplications, nil
}

func (r *ContactRepository) ListOutgoingApplications(userUUID string) ([]*model.ContactApplication, error) {
	var contactApplications []*model.ContactApplication
	if err := r.database().Where("applicant_uuid = ?", userUUID).Order("created_at DESC").Find(&contactApplications).Error; err != nil {
		return nil, fmt.Errorf("list outgoing contact applications: %w", err)
	}
	return contactApplications, nil
}

func (r *ContactRepository) database() *gorm.DB {
	if r != nil && r.db != nil {
		return r.db
	}
	return store.DB
}
