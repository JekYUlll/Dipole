package service

import (
	"errors"
	"testing"

	"github.com/JekYUlll/Dipole/internal/model"
)

type stubUserRepository struct {
	users       map[string]*model.User
	updateCalls int
	updateErr   error
}

func (r *stubUserRepository) GetByUUID(uuid string) (*model.User, error) {
	user, ok := r.users[uuid]
	if !ok {
		return nil, nil
	}

	return user, nil
}

func (r *stubUserRepository) Update(user *model.User) error {
	if r.updateErr != nil {
		return r.updateErr
	}

	r.users[user.UUID] = user
	r.updateCalls++
	return nil
}

func TestUserServiceGetByUUIDNotFound(t *testing.T) {
	t.Parallel()

	service := NewUserService(&stubUserRepository{
		users: map[string]*model.User{},
	})

	_, err := service.GetByUUID("U404")
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserServiceUpdateProfileSelfSuccess(t *testing.T) {
	t.Parallel()

	repo := &stubUserRepository{
		users: map[string]*model.User{
			"U100": {
				UUID:     "U100",
				Nickname: "OldName",
				Email:    "old@example.com",
				Avatar:   "old-avatar",
			},
		},
	}
	service := NewUserService(repo)
	currentUser := repo.users["U100"]
	nickname := "NewName"
	email := "New@Example.com"
	avatar := ""

	updatedUser, err := service.UpdateProfile(currentUser, "U100", UpdateProfileInput{
		Nickname: &nickname,
		Email:    &email,
		Avatar:   &avatar,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if updatedUser.Nickname != "NewName" {
		t.Fatalf("expected nickname updated, got %s", updatedUser.Nickname)
	}
	if updatedUser.Email != "new@example.com" {
		t.Fatalf("expected email normalized, got %s", updatedUser.Email)
	}
	if updatedUser.Avatar != model.DefaultAvatarURL {
		t.Fatalf("expected avatar reset to default, got %s", updatedUser.Avatar)
	}
	if repo.updateCalls != 1 {
		t.Fatalf("expected one update call, got %d", repo.updateCalls)
	}
}

func TestUserServiceUpdateProfileRejectsOtherUser(t *testing.T) {
	t.Parallel()

	repo := &stubUserRepository{
		users: map[string]*model.User{
			"U100": {UUID: "U100", Nickname: "Owner"},
			"U200": {UUID: "U200", Nickname: "Target"},
		},
	}
	service := NewUserService(repo)
	nickname := "Updated"

	_, err := service.UpdateProfile(repo.users["U100"], "U200", UpdateProfileInput{
		Nickname: &nickname,
	})
	if !errors.Is(err, ErrUserPermissionDenied) {
		t.Fatalf("expected ErrUserPermissionDenied, got %v", err)
	}
	if repo.updateCalls != 0 {
		t.Fatalf("expected no update calls, got %d", repo.updateCalls)
	}
}

func TestUserServiceUpdateProfileRejectsInvalidEmail(t *testing.T) {
	t.Parallel()

	repo := &stubUserRepository{
		users: map[string]*model.User{
			"U100": {UUID: "U100", Nickname: "Owner"},
		},
	}
	service := NewUserService(repo)
	email := "invalid-email"

	_, err := service.UpdateProfile(repo.users["U100"], "U100", UpdateProfileInput{
		Email: &email,
	})
	if !errors.Is(err, ErrInvalidEmail) {
		t.Fatalf("expected ErrInvalidEmail, got %v", err)
	}
	if repo.updateCalls != 0 {
		t.Fatalf("expected no update calls, got %d", repo.updateCalls)
	}
}

func TestUserServiceUpdateProfileRejectsEmptyPayload(t *testing.T) {
	t.Parallel()

	repo := &stubUserRepository{
		users: map[string]*model.User{
			"U100": {UUID: "U100", Nickname: "Owner"},
		},
	}
	service := NewUserService(repo)

	_, err := service.UpdateProfile(repo.users["U100"], "U100", UpdateProfileInput{})
	if !errors.Is(err, ErrEmptyProfileUpdate) {
		t.Fatalf("expected ErrEmptyProfileUpdate, got %v", err)
	}
}
