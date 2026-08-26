package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	"github.com/JekYUlll/Dipole/internal/data/mysql/mapper"
	"github.com/JekYUlll/Dipole/internal/model"
)

var _ application.ContactStore = (*ContactRepository)(nil)

type ContactRepository struct {
	queries generated.Querier
}

func NewContactRepository(queries generated.Querier) (*ContactRepository, error) {
	if queries == nil {
		return nil, errors.New("contact queries are required")
	}
	return &ContactRepository{queries: queries}, nil
}

func (r *ContactRepository) AreFriends(userUUID, friendUUID string) (bool, error) {
	contact, err := r.GetContact(userUUID, friendUUID)
	if err != nil {
		return false, fmt.Errorf("check contacts friendship with sqlc: %w", err)
	}
	return contact != nil, nil
}

func (r *ContactRepository) CanSendDirectMessage(userUUID, friendUUID string) (bool, error) {
	contact, err := r.GetContact(userUUID, friendUUID)
	if err != nil {
		return false, fmt.Errorf("get direct message outbound relation with sqlc: %w", err)
	}
	if contact == nil || contact.Status != model.ContactStatusNormal {
		return false, nil
	}
	reverse, err := r.GetContact(friendUUID, userUUID)
	if err != nil {
		return false, fmt.Errorf("get direct message inbound relation with sqlc: %w", err)
	}
	return reverse != nil && reverse.Status == model.ContactStatusNormal, nil
}

func (r *ContactRepository) CreateFriendship(userOneUUID, userTwoUUID string) error {
	_, err := r.queries.CreateFriendship(context.Background(), generated.CreateFriendshipParams{
		UserOneUuid: userOneUUID,
		UserTwoUuid: userTwoUUID,
		Status:      model.ContactStatusNormal,
	})
	if err != nil {
		return fmt.Errorf("create friendship with sqlc: %w", err)
	}
	return nil
}

func (r *ContactRepository) DeleteFriendship(userOneUUID, userTwoUUID string) error {
	_, err := r.queries.DeleteFriendship(context.Background(), generated.DeleteFriendshipParams{
		UserOneUuid: userOneUUID,
		UserTwoUuid: userTwoUUID,
	})
	if err != nil {
		return fmt.Errorf("delete friendship with sqlc: %w", err)
	}
	return nil
}

func (r *ContactRepository) ListFriends(userUUID string) ([]*model.Contact, error) {
	rows, err := r.queries.ListContactsByUser(context.Background(), userUUID)
	if err != nil {
		return nil, fmt.Errorf("list contacts by user UUID with sqlc: %w", err)
	}
	return mapper.Contacts(rows), nil
}

func (r *ContactRepository) GetContact(userUUID, friendUUID string) (*model.Contact, error) {
	if userUUID == "" || friendUUID == "" {
		return nil, nil
	}
	row, err := r.queries.GetContact(context.Background(), generated.GetContactParams{
		UserUuid:   userUUID,
		FriendUuid: friendUUID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get contact with sqlc: %w", err)
	}
	return mapper.Contact(row), nil
}

func (r *ContactRepository) UpdateContact(contact *model.Contact) error {
	if contact == nil {
		return errors.New("update contact with sqlc: contact is required")
	}
	_, err := r.queries.UpdateContact(context.Background(), generated.UpdateContactParams{
		Remark:     contact.Remark,
		Status:     contact.Status,
		UserUuid:   contact.UserUUID,
		FriendUuid: contact.FriendUUID,
	})
	if err != nil {
		return fmt.Errorf("update contact with sqlc: %w", err)
	}
	stored, err := r.GetContact(contact.UserUUID, contact.FriendUUID)
	if err != nil {
		return err
	}
	if stored == nil {
		return errors.New("updated contact was not found")
	}
	*contact = *stored
	return nil
}

func (r *ContactRepository) CreateApplication(contactApplication *model.ContactApplication) error {
	if contactApplication == nil {
		return errors.New("create contact application with sqlc: application is required")
	}
	_, err := r.queries.CreateContactApplication(context.Background(), mapper.ContactApplicationCreateParams(contactApplication))
	if err != nil {
		return fmt.Errorf("create contact application with sqlc: %w", err)
	}
	return r.reloadApplicationByPair(contactApplication)
}

func (r *ContactRepository) GetApplicationByPair(applicantUUID, targetUUID string) (*model.ContactApplication, error) {
	row, err := r.queries.GetContactApplicationByPair(context.Background(), generated.GetContactApplicationByPairParams{
		ApplicantUuid: applicantUUID,
		TargetUuid:    targetUUID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get contact application by pair with sqlc: %w", err)
	}
	return mapper.ContactApplication(row), nil
}

func (r *ContactRepository) GetApplicationByID(id uint) (*model.ContactApplication, error) {
	row, err := r.queries.GetContactApplicationByID(context.Background(), uint64(id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get contact application by ID with sqlc: %w", err)
	}
	return mapper.ContactApplication(row), nil
}

func (r *ContactRepository) UpdateApplication(contactApplication *model.ContactApplication) error {
	if contactApplication == nil {
		return errors.New("update contact application with sqlc: application is required")
	}
	_, err := r.queries.UpdateContactApplication(context.Background(), mapper.ContactApplicationUpdateParams(contactApplication))
	if err != nil {
		return fmt.Errorf("update contact application with sqlc: %w", err)
	}
	stored, err := r.GetApplicationByID(contactApplication.ID)
	if err != nil {
		return err
	}
	if stored == nil {
		return errors.New("updated contact application was not found")
	}
	*contactApplication = *stored
	return nil
}

func (r *ContactRepository) ListIncomingApplications(userUUID string) ([]*model.ContactApplication, error) {
	rows, err := r.queries.ListIncomingContactApplications(context.Background(), userUUID)
	if err != nil {
		return nil, fmt.Errorf("list incoming contact applications with sqlc: %w", err)
	}
	return mapper.ContactApplications(rows), nil
}

func (r *ContactRepository) ListOutgoingApplications(userUUID string) ([]*model.ContactApplication, error) {
	rows, err := r.queries.ListOutgoingContactApplications(context.Background(), userUUID)
	if err != nil {
		return nil, fmt.Errorf("list outgoing contact applications with sqlc: %w", err)
	}
	return mapper.ContactApplications(rows), nil
}

func (r *ContactRepository) reloadApplicationByPair(contactApplication *model.ContactApplication) error {
	stored, err := r.GetApplicationByPair(contactApplication.ApplicantUUID, contactApplication.TargetUUID)
	if err != nil {
		return err
	}
	if stored == nil {
		return errors.New("persisted contact application was not found")
	}
	*contactApplication = *stored
	return nil
}
