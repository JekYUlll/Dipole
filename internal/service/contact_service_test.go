package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
)

type stubContactRepository struct {
	friendships          map[string]map[string]bool
	canSend              map[string]map[string]bool
	contacts             map[string]map[string]*model.Contact
	applications         map[uint]*model.ContactApplication
	nextApplicationID    uint
	incomingApplications []*model.ContactApplication
	outgoingApplications []*model.ContactApplication
}

func newStubContactRepository() *stubContactRepository {
	return &stubContactRepository{
		friendships:       map[string]map[string]bool{},
		canSend:           map[string]map[string]bool{},
		contacts:          map[string]map[string]*model.Contact{},
		applications:      map[uint]*model.ContactApplication{},
		nextApplicationID: 1,
	}
}

func (r *stubContactRepository) AreFriends(userUUID, friendUUID string) (bool, error) {
	return r.friendships[userUUID][friendUUID], nil
}

func (r *stubContactRepository) CanSendDirectMessage(userUUID, friendUUID string) (bool, error) {
	return r.canSend[userUUID][friendUUID], nil
}

func (r *stubContactRepository) CreateFriendship(userOneUUID, userTwoUUID string) error {
	if r.friendships[userOneUUID] == nil {
		r.friendships[userOneUUID] = map[string]bool{}
	}
	if r.friendships[userTwoUUID] == nil {
		r.friendships[userTwoUUID] = map[string]bool{}
	}
	r.friendships[userOneUUID][userTwoUUID] = true
	r.friendships[userTwoUUID][userOneUUID] = true
	if r.canSend[userOneUUID] == nil {
		r.canSend[userOneUUID] = map[string]bool{}
	}
	if r.canSend[userTwoUUID] == nil {
		r.canSend[userTwoUUID] = map[string]bool{}
	}
	r.canSend[userOneUUID][userTwoUUID] = true
	r.canSend[userTwoUUID][userOneUUID] = true
	if r.contacts[userOneUUID] == nil {
		r.contacts[userOneUUID] = map[string]*model.Contact{}
	}
	if r.contacts[userTwoUUID] == nil {
		r.contacts[userTwoUUID] = map[string]*model.Contact{}
	}
	r.contacts[userOneUUID][userTwoUUID] = &model.Contact{UserUUID: userOneUUID, FriendUUID: userTwoUUID, Status: model.ContactStatusNormal, CreatedAt: time.Now().UTC()}
	r.contacts[userTwoUUID][userOneUUID] = &model.Contact{UserUUID: userTwoUUID, FriendUUID: userOneUUID, Status: model.ContactStatusNormal, CreatedAt: time.Now().UTC()}
	return nil
}

func (r *stubContactRepository) DeleteFriendship(userOneUUID, userTwoUUID string) error {
	delete(r.friendships[userOneUUID], userTwoUUID)
	delete(r.friendships[userTwoUUID], userOneUUID)
	delete(r.canSend[userOneUUID], userTwoUUID)
	delete(r.canSend[userTwoUUID], userOneUUID)
	delete(r.contacts[userOneUUID], userTwoUUID)
	delete(r.contacts[userTwoUUID], userOneUUID)
	return nil
}

func (r *stubContactRepository) ListFriends(userUUID string) ([]*model.Contact, error) {
	contacts := make([]*model.Contact, 0, len(r.friendships[userUUID]))
	for friendUUID := range r.friendships[userUUID] {
		contact := r.contacts[userUUID][friendUUID]
		if contact == nil {
			contact = &model.Contact{UserUUID: userUUID, FriendUUID: friendUUID, Status: model.ContactStatusNormal, CreatedAt: time.Now().UTC()}
		}
		contacts = append(contacts, contact)
	}
	return contacts, nil
}

func (r *stubContactRepository) GetContact(userUUID, friendUUID string) (*model.Contact, error) {
	return r.contacts[userUUID][friendUUID], nil
}

func (r *stubContactRepository) UpdateContact(contact *model.Contact) error {
	if r.contacts[contact.UserUUID] == nil {
		r.contacts[contact.UserUUID] = map[string]*model.Contact{}
	}
	r.contacts[contact.UserUUID][contact.FriendUUID] = contact
	if r.canSend[contact.UserUUID] == nil {
		r.canSend[contact.UserUUID] = map[string]bool{}
	}
	r.canSend[contact.UserUUID][contact.FriendUUID] = contact.Status == model.ContactStatusNormal
	return nil
}

func (r *stubContactRepository) CreateApplication(application *model.ContactApplication) error {
	application.ID = r.nextApplicationID
	r.nextApplicationID++
	r.applications[application.ID] = application
	return nil
}

func (r *stubContactRepository) GetApplicationByPair(applicantUUID, targetUUID string) (*model.ContactApplication, error) {
	for _, application := range r.applications {
		if application.ApplicantUUID == applicantUUID && application.TargetUUID == targetUUID {
			return application, nil
		}
	}
	return nil, nil
}

func (r *stubContactRepository) GetApplicationByID(id uint) (*model.ContactApplication, error) {
	return r.applications[id], nil
}

func (r *stubContactRepository) UpdateApplication(application *model.ContactApplication) error {
	r.applications[application.ID] = application
	return nil
}

func (r *stubContactRepository) ListIncomingApplications(userUUID string) ([]*model.ContactApplication, error) {
	if r.incomingApplications != nil {
		return r.incomingApplications, nil
	}

	var applications []*model.ContactApplication
	for _, application := range r.applications {
		if application.TargetUUID == userUUID {
			applications = append(applications, application)
		}
	}
	return applications, nil
}

func (r *stubContactRepository) ListOutgoingApplications(userUUID string) ([]*model.ContactApplication, error) {
	if r.outgoingApplications != nil {
		return r.outgoingApplications, nil
	}

	var applications []*model.ContactApplication
	for _, application := range r.applications {
		if application.ApplicantUUID == userUUID {
			applications = append(applications, application)
		}
	}
	return applications, nil
}

type stubContactUserFinder struct {
	users map[string]*model.User
	err   error
}

type stubContactNotifier struct {
	notifications []struct {
		userUUID   string
		friendUUID string
	}
}

func (n *stubContactNotifier) NotifyFriendDeleted(userUUID, friendUUID string, occurredAt time.Time) {
	n.notifications = append(n.notifications, struct {
		userUUID   string
		friendUUID string
	}{
		userUUID:   userUUID,
		friendUUID: friendUUID,
	})
}

type stubContactEventPublisher struct {
	published []struct {
		topic     string
		eventType string
		key       string
		payload   any
	}
}

func (p *stubContactEventPublisher) PublishJSON(ctx context.Context, topic string, key string, payload any, headers map[string]string) error {
	return nil
}

func (p *stubContactEventPublisher) PublishEvent(ctx context.Context, topic string, key string, eventType string, payload any, headers map[string]string) error {
	p.published = append(p.published, struct {
		topic     string
		eventType string
		key       string
		payload   any
	}{
		topic:     topic,
		eventType: eventType,
		key:       key,
		payload:   payload,
	})
	return nil
}

func (f *stubContactUserFinder) GetByUUID(uuid string) (*model.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.users[uuid], nil
}

func (f *stubContactUserFinder) ListByUUIDs(uuids []string) ([]*model.User, error) {
	if f.err != nil {
		return nil, f.err
	}

	users := make([]*model.User, 0, len(uuids))
	seen := map[string]struct{}{}
	for _, uuid := range uuids {
		if _, ok := seen[uuid]; ok {
			continue
		}
		seen[uuid] = struct{}{}
		if user := f.users[uuid]; user != nil {
			users = append(users, user)
		}
	}
	return users, nil
}

func TestContactServiceApplySuccess(t *testing.T) {
	t.Parallel()

	repo := newStubContactRepository()
	service := NewContactService(repo, &stubContactUserFinder{
		users: map[string]*model.User{
			"U200": {UUID: "U200", Status: model.UserStatusNormal},
		},
	})

	application, err := service.Apply("U100", ApplyContactInput{TargetUUID: "U200", Message: "hello"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if application.ID == 0 {
		t.Fatalf("expected persisted application id")
	}
	if application.Status != model.ContactApplicationPending {
		t.Fatalf("expected pending status, got %d", application.Status)
	}
	if application.ExpiresAt == nil {
		t.Fatalf("expected expires_at to be set")
	}
}

func TestContactServiceApplyRejectsDuplicatePending(t *testing.T) {
	t.Parallel()

	repo := newStubContactRepository()
	_ = repo.CreateApplication(&model.ContactApplication{
		ApplicantUUID: "U100",
		TargetUUID:    "U200",
		Status:        model.ContactApplicationPending,
	})
	service := NewContactService(repo, &stubContactUserFinder{
		users: map[string]*model.User{
			"U200": {UUID: "U200", Status: model.UserStatusNormal},
		},
	})

	_, err := service.Apply("U100", ApplyContactInput{TargetUUID: "U200"})
	if !errors.Is(err, ErrContactApplicationExists) {
		t.Fatalf("expected ErrContactApplicationExists, got %v", err)
	}
}

func TestContactServiceHandleApplicationAcceptSuccess(t *testing.T) {
	t.Parallel()

	repo := newStubContactRepository()
	application := &model.ContactApplication{
		ApplicantUUID: "U100",
		TargetUUID:    "U200",
		Status:        model.ContactApplicationPending,
		ExpiresAt:     ptrTime(time.Now().UTC().Add(24 * time.Hour)),
	}
	_ = repo.CreateApplication(application)
	service := NewContactService(repo, &stubContactUserFinder{})

	updated, err := service.HandleApplication("U200", application.ID, ContactActionAccept)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if updated.Status != model.ContactApplicationAccepted {
		t.Fatalf("expected accepted status, got %d", updated.Status)
	}
	isFriend, _ := repo.AreFriends("U100", "U200")
	if !isFriend {
		t.Fatalf("expected friendship to be created")
	}
}

func TestContactServiceHandleApplicationRejectsExpired(t *testing.T) {
	t.Parallel()

	repo := newStubContactRepository()
	application := &model.ContactApplication{
		ApplicantUUID: "U100",
		TargetUUID:    "U200",
		Status:        model.ContactApplicationPending,
		CreatedAt:     time.Now().UTC().Add(-8 * 24 * time.Hour),
	}
	_ = repo.CreateApplication(application)
	service := NewContactService(repo, &stubContactUserFinder{})

	_, err := service.HandleApplication("U200", application.ID, ContactActionAccept)
	if !errors.Is(err, ErrContactApplicationExpired) {
		t.Fatalf("expected ErrContactApplicationExpired, got %v", err)
	}
	if repo.applications[application.ID].Status != model.ContactApplicationExpired {
		t.Fatalf("expected expired status, got %d", repo.applications[application.ID].Status)
	}
}

func TestContactServiceListIncomingApplicationsMarksExpired(t *testing.T) {
	t.Parallel()

	repo := newStubContactRepository()
	application := &model.ContactApplication{
		ID:            1,
		ApplicantUUID: "U100",
		TargetUUID:    "U200",
		Status:        model.ContactApplicationPending,
		CreatedAt:     time.Now().UTC().Add(-8 * 24 * time.Hour),
	}
	repo.applications[application.ID] = application
	repo.incomingApplications = []*model.ContactApplication{application}
	service := NewContactService(repo, &stubContactUserFinder{
		users: map[string]*model.User{
			"U100": {UUID: "U100"},
			"U200": {UUID: "U200"},
		},
	})

	items, err := service.ListIncomingApplications("U200")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 application, got %d", len(items))
	}
	if items[0].Application.Status != model.ContactApplicationExpired {
		t.Fatalf("expected expired application status, got %d", items[0].Application.Status)
	}
}

func ptrTime(value time.Time) *time.Time {
	return &value
}

func TestContactServiceListFriendsSuccess(t *testing.T) {
	t.Parallel()

	repo := newStubContactRepository()
	_ = repo.CreateFriendship("U100", "U200")
	service := NewContactService(repo, &stubContactUserFinder{
		users: map[string]*model.User{
			"U200": {UUID: "U200", Nickname: "Alice", Status: model.UserStatusNormal},
		},
	})

	items, err := service.ListFriends("U100")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 friend, got %d", len(items))
	}
	if items[0].User.UUID != "U200" {
		t.Fatalf("expected friend U200, got %s", items[0].User.UUID)
	}
}

func TestContactServiceUpdateRemarkSuccess(t *testing.T) {
	t.Parallel()

	repo := newStubContactRepository()
	_ = repo.CreateFriendship("U100", "U200")
	service := NewContactService(repo, &stubContactUserFinder{})

	contact, err := service.UpdateRemark("U100", "U200", "teammate")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if contact.Remark != "teammate" {
		t.Fatalf("expected remark updated, got %s", contact.Remark)
	}
}

func TestContactServiceUpdateBlockStatusSuccess(t *testing.T) {
	t.Parallel()

	repo := newStubContactRepository()
	_ = repo.CreateFriendship("U100", "U200")
	service := NewContactService(repo, &stubContactUserFinder{})

	contact, err := service.UpdateBlockStatus("U100", "U200", true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if contact.Status != model.ContactStatusBlocked {
		t.Fatalf("expected blocked status, got %d", contact.Status)
	}
	if repo.canSend["U100"]["U200"] {
		t.Fatalf("expected blocked contact to disable direct messaging")
	}
}

func TestContactServiceDeleteFriendRejectsMissingFriendship(t *testing.T) {
	t.Parallel()

	service := NewContactService(newStubContactRepository(), &stubContactUserFinder{})

	err := service.DeleteFriend("U100", "U200")
	if !errors.Is(err, ErrContactTargetNotFound) {
		t.Fatalf("expected ErrContactTargetNotFound, got %v", err)
	}
}

func TestContactServiceDeleteFriendNotifiesBothSidesWithoutEvents(t *testing.T) {
	t.Parallel()

	repo := newStubContactRepository()
	_ = repo.CreateFriendship("U100", "U200")
	notifier := &stubContactNotifier{}
	service := NewContactService(repo, &stubContactUserFinder{}).WithNotifier(notifier)

	if err := service.DeleteFriend("U100", "U200"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(notifier.notifications) != 2 {
		t.Fatalf("expected 2 notifications, got %d", len(notifier.notifications))
	}
}

func TestContactServiceDeleteFriendPublishesBothSideEvents(t *testing.T) {
	t.Parallel()

	repo := newStubContactRepository()
	_ = repo.CreateFriendship("U100", "U200")
	publisher := &stubContactEventPublisher{}
	service := NewContactService(repo, &stubContactUserFinder{}).WithEvents(publisher)

	if err := service.DeleteFriend("U100", "U200"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(publisher.published) != 2 {
		t.Fatalf("expected 2 published events, got %d", len(publisher.published))
	}
	for _, item := range publisher.published {
		if item.topic != "contact.friend.deleted" {
			t.Fatalf("unexpected topic: %s", item.topic)
		}
		if item.eventType != "contact.friend.deleted" {
			t.Fatalf("unexpected event type: %s", item.eventType)
		}
	}
}
