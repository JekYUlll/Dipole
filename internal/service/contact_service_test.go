package service

import (
	"errors"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
)

type stubContactRepository struct {
	friendships          map[string]map[string]bool
	applications         map[uint]*model.ContactApplication
	nextApplicationID    uint
	incomingApplications []*model.ContactApplication
	outgoingApplications []*model.ContactApplication
}

func newStubContactRepository() *stubContactRepository {
	return &stubContactRepository{
		friendships:       map[string]map[string]bool{},
		applications:      map[uint]*model.ContactApplication{},
		nextApplicationID: 1,
	}
}

func (r *stubContactRepository) AreFriends(userUUID, friendUUID string) (bool, error) {
	return r.friendships[userUUID][friendUUID], nil
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
	return nil
}

func (r *stubContactRepository) DeleteFriendship(userOneUUID, userTwoUUID string) error {
	delete(r.friendships[userOneUUID], userTwoUUID)
	delete(r.friendships[userTwoUUID], userOneUUID)
	return nil
}

func (r *stubContactRepository) ListFriends(userUUID string) ([]*model.Contact, error) {
	contacts := make([]*model.Contact, 0, len(r.friendships[userUUID]))
	for friendUUID := range r.friendships[userUUID] {
		contacts = append(contacts, &model.Contact{
			UserUUID:   userUUID,
			FriendUUID: friendUUID,
			CreatedAt:  time.Now().UTC(),
		})
	}
	return contacts, nil
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

func TestContactServiceDeleteFriendRejectsMissingFriendship(t *testing.T) {
	t.Parallel()

	service := NewContactService(newStubContactRepository(), &stubContactUserFinder{})

	err := service.DeleteFriend("U100", "U200")
	if !errors.Is(err, ErrContactTargetNotFound) {
		t.Fatalf("expected ErrContactTargetNotFound, got %v", err)
	}
}
