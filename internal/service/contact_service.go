package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
)

var (
	ErrContactTargetRequired      = errors.New("contact target is required")
	ErrContactTargetNotFound      = errors.New("contact target not found")
	ErrContactTargetUnavailable   = errors.New("contact target is unavailable")
	ErrContactCannotAddSelf       = errors.New("cannot add self as contact")
	ErrContactAlreadyFriends      = errors.New("users are already friends")
	ErrContactApplicationExists   = errors.New("contact application already exists")
	ErrContactApplicationNotFound = errors.New("contact application not found")
	ErrContactApplicationHandled  = errors.New("contact application has been handled")
	ErrContactApplicationExpired  = errors.New("contact application has expired")
	ErrContactPermissionDenied    = errors.New("contact permission denied")
	ErrContactActionInvalid       = errors.New("contact action is invalid")
	ErrContactRemarkTooLong       = errors.New("contact remark is too long")
)

const (
	ContactActionAccept   = "accept"
	ContactActionReject   = "reject"
	contactApplicationTTL = 7 * 24 * time.Hour
)

type contactRepository interface {
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
}

type contactUserFinder interface {
	GetByUUID(uuid string) (*model.User, error)
	ListByUUIDs(uuids []string) ([]*model.User, error)
}

type ApplyContactInput struct {
	TargetUUID string
	Message    string
}

type ContactListItem struct {
	User      *model.User
	Remark    string
	Status    int8
	CreatedAt time.Time
}

type ContactApplicationView struct {
	Application *model.ContactApplication
	Applicant   *model.User
	Target      *model.User
}

type ContactService struct {
	repo       contactRepository
	userFinder contactUserFinder
	notifier   contactNotifier
	events     eventPublisher
}

type contactNotifier interface {
	NotifyFriendDeleted(userUUID, friendUUID string, occurredAt time.Time)
}

type ContactFriendDeletedPayload struct {
	UserUUID   string    `json:"user_uuid"`
	FriendUUID string    `json:"friend_uuid"`
	OccurredAt time.Time `json:"occurred_at"`
}

func NewContactService(repo contactRepository, userFinder contactUserFinder) *ContactService {
	return &ContactService{
		repo:       repo,
		userFinder: userFinder,
	}
}

func (s *ContactService) WithNotifier(notifier contactNotifier) *ContactService {
	s.notifier = notifier
	return s
}

func (s *ContactService) WithEvents(events eventPublisher) *ContactService {
	s.events = events
	return s
}

func (s *ContactService) Apply(currentUserUUID string, input ApplyContactInput) (*model.ContactApplication, error) {
	targetUUID := strings.TrimSpace(input.TargetUUID)
	message := strings.TrimSpace(input.Message)
	if targetUUID == "" {
		return nil, ErrContactTargetRequired
	}
	if currentUserUUID == targetUUID {
		return nil, ErrContactCannotAddSelf
	}

	targetUser, err := s.userFinder.GetByUUID(targetUUID)
	if err != nil {
		return nil, fmt.Errorf("get target user in apply contact: %w", err)
	}
	if targetUser == nil {
		return nil, ErrContactTargetNotFound
	}
	if targetUser.Status != model.UserStatusNormal {
		return nil, ErrContactTargetUnavailable
	}

	isFriend, err := s.repo.AreFriends(currentUserUUID, targetUUID)
	if err != nil {
		return nil, fmt.Errorf("check friendship in apply contact: %w", err)
	}
	if isFriend {
		return nil, ErrContactAlreadyFriends
	}

	existingOutgoing, err := s.repo.GetApplicationByPair(currentUserUUID, targetUUID)
	if err != nil {
		return nil, fmt.Errorf("get outgoing application in apply contact: %w", err)
	}
	if err := s.markExpiredIfNeeded(existingOutgoing); err != nil {
		return nil, fmt.Errorf("refresh outgoing application expiration in apply contact: %w", err)
	}
	if existingOutgoing != nil && existingOutgoing.Status == model.ContactApplicationPending {
		return nil, ErrContactApplicationExists
	}

	existingIncoming, err := s.repo.GetApplicationByPair(targetUUID, currentUserUUID)
	if err != nil {
		return nil, fmt.Errorf("get incoming application in apply contact: %w", err)
	}
	if err := s.markExpiredIfNeeded(existingIncoming); err != nil {
		return nil, fmt.Errorf("refresh incoming application expiration in apply contact: %w", err)
	}
	if existingIncoming != nil && existingIncoming.Status == model.ContactApplicationPending {
		return nil, ErrContactApplicationExists
	}

	now := time.Now().UTC()
	expiresAt := now.Add(contactApplicationTTL)
	application := &model.ContactApplication{
		ApplicantUUID: currentUserUUID,
		TargetUUID:    targetUUID,
		Message:       truncateContactMessage(message),
		Status:        model.ContactApplicationPending,
		ExpiresAt:     &expiresAt,
	}

	if existingOutgoing != nil {
		existingOutgoing.Message = application.Message
		existingOutgoing.Status = model.ContactApplicationPending
		existingOutgoing.ExpiresAt = application.ExpiresAt
		existingOutgoing.HandledAt = nil
		if err := s.repo.UpdateApplication(existingOutgoing); err != nil {
			return nil, fmt.Errorf("reset contact application in apply contact: %w", err)
		}
		return existingOutgoing, nil
	}

	if err := s.repo.CreateApplication(application); err != nil {
		return nil, fmt.Errorf("create contact application in apply contact: %w", err)
	}

	return application, nil
}

func (s *ContactService) ListFriends(currentUserUUID string) ([]*ContactListItem, error) {
	contacts, err := s.repo.ListFriends(currentUserUUID)
	if err != nil {
		return nil, fmt.Errorf("list friends: %w", err)
	}

	friendUUIDs := make([]string, 0, len(contacts))
	for _, contact := range contacts {
		friendUUIDs = append(friendUUIDs, contact.FriendUUID)
	}

	users, err := s.userFinder.ListByUUIDs(friendUUIDs)
	if err != nil {
		return nil, fmt.Errorf("list friend users: %w", err)
	}

	usersByUUID := make(map[string]*model.User, len(users))
	for _, user := range users {
		usersByUUID[user.UUID] = user
	}

	items := make([]*ContactListItem, 0, len(contacts))
	for _, contact := range contacts {
		user := usersByUUID[contact.FriendUUID]
		if user == nil {
			continue
		}
		items = append(items, &ContactListItem{
			User:      user,
			Remark:    contact.Remark,
			Status:    contact.Status,
			CreatedAt: contact.CreatedAt,
		})
	}

	return items, nil
}

func (s *ContactService) UpdateRemark(currentUserUUID, friendUUID, remark string) (*model.Contact, error) {
	friendUUID = strings.TrimSpace(friendUUID)
	if friendUUID == "" {
		return nil, ErrContactTargetRequired
	}

	contact, err := s.repo.GetContact(currentUserUUID, friendUUID)
	if err != nil {
		return nil, fmt.Errorf("get contact in update remark: %w", err)
	}
	if contact == nil {
		return nil, ErrContactTargetNotFound
	}

	remark = strings.TrimSpace(remark)
	if len([]rune(remark)) > 50 {
		return nil, ErrContactRemarkTooLong
	}

	contact.Remark = remark
	if err := s.repo.UpdateContact(contact); err != nil {
		return nil, fmt.Errorf("update contact remark: %w", err)
	}

	return contact, nil
}

func (s *ContactService) UpdateBlockStatus(currentUserUUID, friendUUID string, blocked bool) (*model.Contact, error) {
	friendUUID = strings.TrimSpace(friendUUID)
	if friendUUID == "" {
		return nil, ErrContactTargetRequired
	}

	contact, err := s.repo.GetContact(currentUserUUID, friendUUID)
	if err != nil {
		return nil, fmt.Errorf("get contact in update block status: %w", err)
	}
	if contact == nil {
		return nil, ErrContactTargetNotFound
	}

	if blocked {
		contact.Status = model.ContactStatusBlocked
	} else {
		contact.Status = model.ContactStatusNormal
	}

	if err := s.repo.UpdateContact(contact); err != nil {
		return nil, fmt.Errorf("update contact block status: %w", err)
	}

	return contact, nil
}

func (s *ContactService) ListIncomingApplications(currentUserUUID string) ([]*ContactApplicationView, error) {
	applications, err := s.repo.ListIncomingApplications(currentUserUUID)
	if err != nil {
		return nil, fmt.Errorf("list incoming contact applications: %w", err)
	}
	if err := s.refreshApplicationsExpiration(applications); err != nil {
		return nil, fmt.Errorf("refresh incoming contact applications expiration: %w", err)
	}

	return s.buildApplicationViews(applications)
}

func (s *ContactService) ListOutgoingApplications(currentUserUUID string) ([]*ContactApplicationView, error) {
	applications, err := s.repo.ListOutgoingApplications(currentUserUUID)
	if err != nil {
		return nil, fmt.Errorf("list outgoing contact applications: %w", err)
	}
	if err := s.refreshApplicationsExpiration(applications); err != nil {
		return nil, fmt.Errorf("refresh outgoing contact applications expiration: %w", err)
	}

	return s.buildApplicationViews(applications)
}

func (s *ContactService) HandleApplication(currentUserUUID string, applicationID uint, action string) (*model.ContactApplication, error) {
	application, err := s.repo.GetApplicationByID(applicationID)
	if err != nil {
		return nil, fmt.Errorf("get contact application in handle application: %w", err)
	}
	if application == nil {
		return nil, ErrContactApplicationNotFound
	}
	if err := s.markExpiredIfNeeded(application); err != nil {
		return nil, fmt.Errorf("refresh contact application expiration in handle application: %w", err)
	}
	if application.TargetUUID != currentUserUUID {
		return nil, ErrContactPermissionDenied
	}
	if application.Status == model.ContactApplicationExpired {
		return nil, ErrContactApplicationExpired
	}
	if application.Status != model.ContactApplicationPending {
		return nil, ErrContactApplicationHandled
	}

	now := time.Now().UTC()
	switch action {
	case ContactActionAccept:
		if err := s.repo.CreateFriendship(application.ApplicantUUID, application.TargetUUID); err != nil {
			return nil, fmt.Errorf("create friendship in handle application: %w", err)
		}
		application.Status = model.ContactApplicationAccepted
	case ContactActionReject:
		application.Status = model.ContactApplicationRejected
	default:
		return nil, ErrContactActionInvalid
	}

	application.HandledAt = &now
	if err := s.repo.UpdateApplication(application); err != nil {
		return nil, fmt.Errorf("update application in handle application: %w", err)
	}

	return application, nil
}

func (s *ContactService) DeleteFriend(currentUserUUID, friendUUID string) error {
	friendUUID = strings.TrimSpace(friendUUID)
	if friendUUID == "" {
		return ErrContactTargetRequired
	}

	isFriend, err := s.repo.AreFriends(currentUserUUID, friendUUID)
	if err != nil {
		return fmt.Errorf("check friendship in delete friend: %w", err)
	}
	if !isFriend {
		return ErrContactTargetNotFound
	}

	if err := s.repo.DeleteFriendship(currentUserUUID, friendUUID); err != nil {
		return fmt.Errorf("delete friendship in delete friend: %w", err)
	}

	occurredAt := time.Now().UTC()
	if s.events != nil {
		payload := ContactFriendDeletedPayload{
			UserUUID:   currentUserUUID,
			FriendUUID: friendUUID,
			OccurredAt: occurredAt,
		}
		if err := s.events.PublishEvent(context.Background(), "dipole.contact.friend.deleted", currentUserUUID, "contact.friend.deleted", payload, nil); err != nil {
			return fmt.Errorf("publish contact friend deleted event for user: %w", err)
		}
		reversePayload := ContactFriendDeletedPayload{
			UserUUID:   friendUUID,
			FriendUUID: currentUserUUID,
			OccurredAt: occurredAt,
		}
		if err := s.events.PublishEvent(context.Background(), "dipole.contact.friend.deleted", friendUUID, "contact.friend.deleted", reversePayload, nil); err != nil {
			return fmt.Errorf("publish contact friend deleted event for friend: %w", err)
		}
	} else if s.notifier != nil {
		s.notifier.NotifyFriendDeleted(currentUserUUID, friendUUID, occurredAt)
		s.notifier.NotifyFriendDeleted(friendUUID, currentUserUUID, occurredAt)
	}

	return nil
}

func (s *ContactService) buildApplicationViews(applications []*model.ContactApplication) ([]*ContactApplicationView, error) {
	userUUIDs := make([]string, 0, len(applications)*2)
	seen := make(map[string]struct{}, len(applications)*2)
	for _, application := range applications {
		if _, ok := seen[application.ApplicantUUID]; !ok {
			seen[application.ApplicantUUID] = struct{}{}
			userUUIDs = append(userUUIDs, application.ApplicantUUID)
		}
		if _, ok := seen[application.TargetUUID]; !ok {
			seen[application.TargetUUID] = struct{}{}
			userUUIDs = append(userUUIDs, application.TargetUUID)
		}
	}

	users, err := s.userFinder.ListByUUIDs(userUUIDs)
	if err != nil {
		return nil, fmt.Errorf("list users for contact applications: %w", err)
	}

	usersByUUID := make(map[string]*model.User, len(users))
	for _, user := range users {
		usersByUUID[user.UUID] = user
	}

	views := make([]*ContactApplicationView, 0, len(applications))
	for _, application := range applications {
		views = append(views, &ContactApplicationView{
			Application: application,
			Applicant:   usersByUUID[application.ApplicantUUID],
			Target:      usersByUUID[application.TargetUUID],
		})
	}

	return views, nil
}

func (s *ContactService) refreshApplicationsExpiration(applications []*model.ContactApplication) error {
	for _, application := range applications {
		if err := s.markExpiredIfNeeded(application); err != nil {
			return err
		}
	}

	return nil
}

func (s *ContactService) markExpiredIfNeeded(application *model.ContactApplication) error {
	if application == nil || application.Status != model.ContactApplicationPending {
		return nil
	}

	expiresAt := applicationExpiryTime(application)
	if expiresAt == nil || expiresAt.After(time.Now().UTC()) {
		if application.ExpiresAt == nil && expiresAt != nil {
			application.ExpiresAt = expiresAt
			if err := s.repo.UpdateApplication(application); err != nil {
				return fmt.Errorf("backfill application expires_at: %w", err)
			}
		}
		return nil
	}

	now := time.Now().UTC()
	application.Status = model.ContactApplicationExpired
	application.ExpiresAt = expiresAt
	application.HandledAt = &now
	if err := s.repo.UpdateApplication(application); err != nil {
		return fmt.Errorf("update expired application: %w", err)
	}

	return nil
}

func applicationExpiryTime(application *model.ContactApplication) *time.Time {
	if application == nil {
		return nil
	}
	if application.ExpiresAt != nil {
		exp := application.ExpiresAt.UTC()
		return &exp
	}
	if application.CreatedAt.IsZero() {
		return nil
	}

	exp := application.CreatedAt.UTC().Add(contactApplicationTTL)
	return &exp
}

func truncateContactMessage(message string) string {
	runes := []rune(message)
	if len(runes) <= 255 {
		return message
	}

	return string(runes[:255])
}
