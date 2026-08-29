package service

import (
	"github.com/JekYUlll/Dipole/internal/model"
	corecontact "github.com/JekYUlll/Dipole/internal/services/core/domain/contact"
)

var (
	ErrContactTargetRequired      = corecontact.ErrContactTargetRequired
	ErrContactTargetNotFound      = corecontact.ErrContactTargetNotFound
	ErrContactTargetUnavailable   = corecontact.ErrContactTargetUnavailable
	ErrContactCannotAddSelf       = corecontact.ErrContactCannotAddSelf
	ErrContactAlreadyFriends      = corecontact.ErrContactAlreadyFriends
	ErrContactApplicationExists   = corecontact.ErrContactApplicationExists
	ErrContactApplicationNotFound = corecontact.ErrContactApplicationNotFound
	ErrContactApplicationHandled  = corecontact.ErrContactApplicationHandled
	ErrContactApplicationExpired  = corecontact.ErrContactApplicationExpired
	ErrContactPermissionDenied    = corecontact.ErrContactPermissionDenied
	ErrContactActionInvalid       = corecontact.ErrContactActionInvalid
	ErrContactRemarkTooLong       = corecontact.ErrContactRemarkTooLong
)

const (
	ContactActionAccept = corecontact.ContactActionAccept
	ContactActionReject = corecontact.ContactActionReject
)

type ApplyContactInput = corecontact.ApplyContactInput
type ContactListItem = corecontact.ContactListItem
type ContactApplicationView = corecontact.ContactApplicationView
type ContactFriendDeletedPayload = corecontact.ContactFriendDeletedPayload
type ContactService = corecontact.ContactService

func NewContactService(repo interface {
	AreFriends(userUUID, friendUUID string) (bool, error)
	CanSendDirectMessage(userUUID, friendUUID string) (bool, error)
	CreateFriendship(userOneUUID, userTwoUUID string) error
	DeleteFriendship(userOneUUID, userTwoUUID string) error
	ListFriends(userUUID string) ([]*model.Contact, error)
	GetContact(userUUID, friendUUID string) (*model.Contact, error)
	UpdateContact(contact *model.Contact) error
	CreateApplication(application *model.ContactApplication) error
	GetApplicationByPair(applicantUUID, targetUUID string) (*model.ContactApplication, error)
	GetApplicationByID(id uint) (*model.ContactApplication, error)
	UpdateApplication(application *model.ContactApplication) error
	ListIncomingApplications(userUUID string) ([]*model.ContactApplication, error)
	ListOutgoingApplications(userUUID string) ([]*model.ContactApplication, error)
}, userFinder interface {
	GetByUUID(uuid string) (*model.User, error)
	ListByUUIDs(uuids []string) ([]*model.User, error)
}) *ContactService {
	return corecontact.NewContactService(repo, userFinder)
}
