package http

import (
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/service"
)

func TestPresentUserForViewerReturnsPublicProfileForOtherUser(t *testing.T) {
	t.Parallel()

	viewer := &model.User{UUID: "U100"}
	target := &model.User{
		ID:        2,
		UUID:      "U200",
		Nickname:  "Target",
		Telephone: "13800000000",
		Email:     "target@example.com",
		Avatar:    "avatar",
		Status:    model.UserStatusNormal,
	}

	got := presentUserForViewer(viewer, target)
	response, ok := got.(*publicUserResponse)
	if !ok {
		t.Fatalf("expected publicUserResponse, got %T", got)
	}
	if response.UUID != target.UUID {
		t.Fatalf("expected uuid %s, got %s", target.UUID, response.UUID)
	}
	if response.Nickname != target.Nickname {
		t.Fatalf("expected nickname %s, got %s", target.Nickname, response.Nickname)
	}
}

func TestPresentUserForViewerReturnsPrivateProfileForSelf(t *testing.T) {
	t.Parallel()

	user := &model.User{
		ID:        1,
		UUID:      "U100",
		Nickname:  "Self",
		Telephone: "13800000000",
		Email:     "self@example.com",
		Avatar:    "avatar",
		IsAdmin:   false,
		Status:    model.UserStatusNormal,
	}

	got := presentUserForViewer(user, user)
	response, ok := got.(*privateUserResponse)
	if !ok {
		t.Fatalf("expected privateUserResponse, got %T", got)
	}
	if response.Telephone != user.Telephone {
		t.Fatalf("expected telephone %s, got %s", user.Telephone, response.Telephone)
	}
	if response.Email != user.Email {
		t.Fatalf("expected email %s, got %s", user.Email, response.Email)
	}
}

func TestPresentUserForViewerReturnsPrivateProfileForAdmin(t *testing.T) {
	t.Parallel()

	viewer := &model.User{
		UUID:    "U999",
		IsAdmin: true,
	}
	target := &model.User{
		ID:        2,
		UUID:      "U200",
		Telephone: "13800000000",
		Email:     "target@example.com",
	}

	got := presentUserForViewer(viewer, target)
	response, ok := got.(*privateUserResponse)
	if !ok {
		t.Fatalf("expected privateUserResponse, got %T", got)
	}
	if response.Telephone != target.Telephone {
		t.Fatalf("expected telephone %s, got %s", target.Telephone, response.Telephone)
	}
}

func TestNewAuthResponseUsesPrivateProfile(t *testing.T) {
	t.Parallel()

	now := time.Now()
	result := &service.AuthResult{
		Token: "TOKEN123",
		User: &model.User{
			ID:        1,
			UUID:      "U100",
			Nickname:  "Self",
			Telephone: "13800000000",
			Email:     "self@example.com",
			Avatar:    "avatar",
			IsAdmin:   true,
			Status:    model.UserStatusNormal,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	response := newAuthResponse(result)
	if response.Token != result.Token {
		t.Fatalf("expected token %s, got %s", result.Token, response.Token)
	}
	if response.User == nil {
		t.Fatal("expected user response")
	}
	if response.User.Telephone != result.User.Telephone {
		t.Fatalf("expected telephone %s, got %s", result.User.Telephone, response.User.Telephone)
	}
}
