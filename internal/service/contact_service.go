package service

import (
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
	ErrContactPermissionDenied    = errors.New("contact permission denied")
	ErrContactActionInvalid       = errors.New("contact action is invalid")
)

const (
	ContactActionAccept = "accept"
	ContactActionReject = "reject"
)

type contactRepository interface {
	AreFriends(userUUID, friendUUID string) (bool, error)
	CreateFriendship(userOneUUID, userTwoUUID string) error
	DeleteFriendship(userOneUUID, userTwoUUID string) error
	ListFriends(userUUID string) ([]*model.Contact, error)
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
}

func NewContactService(repo contactRepository, userFinder contactUserFinder) *ContactService {
	return &ContactService{
		repo:       repo,
		userFinder: userFinder,
	}
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
	if existingOutgoing != nil && existingOutgoing.Status == model.ContactApplicationPending {
		return nil, ErrContactApplicationExists
	}

	existingIncoming, err := s.repo.GetApplicationByPair(targetUUID, currentUserUUID)
	if err != nil {
		return nil, fmt.Errorf("get incoming application in apply contact: %w", err)
	}
	if existingIncoming != nil && existingIncoming.Status == model.ContactApplicationPending {
		return nil, ErrContactApplicationExists
	}

	application := &model.ContactApplication{
		ApplicantUUID: currentUserUUID,
		TargetUUID:    targetUUID,
		Message:       truncateContactMessage(message),
		Status:        model.ContactApplicationPending,
	}

	if existingOutgoing != nil {
		existingOutgoing.Message = application.Message
		existingOutgoing.Status = model.ContactApplicationPending
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
			CreatedAt: contact.CreatedAt,
		})
	}

	return items, nil
}

func (s *ContactService) ListIncomingApplications(currentUserUUID string) ([]*ContactApplicationView, error) {
	applications, err := s.repo.ListIncomingApplications(currentUserUUID)
	if err != nil {
		return nil, fmt.Errorf("list incoming contact applications: %w", err)
	}

	return s.buildApplicationViews(applications)
}

func (s *ContactService) ListOutgoingApplications(currentUserUUID string) ([]*ContactApplicationView, error) {
	applications, err := s.repo.ListOutgoingApplications(currentUserUUID)
	if err != nil {
		return nil, fmt.Errorf("list outgoing contact applications: %w", err)
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
	if application.TargetUUID != currentUserUUID {
		return nil, ErrContactPermissionDenied
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

func truncateContactMessage(message string) string {
	runes := []rune(message)
	if len(runes) <= 255 {
		return message
	}

	return string(runes[:255])
}
