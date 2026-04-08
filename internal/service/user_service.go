package service

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"unicode/utf8"

	"github.com/JekYUlll/Dipole/internal/model"
)

var (
	ErrUserNotFound         = errors.New("user not found")
	ErrUserPermissionDenied = errors.New("user permission denied")
	ErrEmptyProfileUpdate   = errors.New("empty profile update")
	ErrInvalidNickname      = errors.New("invalid nickname")
	ErrInvalidEmail         = errors.New("invalid email")
	ErrInvalidAvatar        = errors.New("invalid avatar")
)

type userRepository interface {
	GetByUUID(uuid string) (*model.User, error)
	Update(user *model.User) error
}

type UpdateProfileInput struct {
	Nickname *string `json:"nickname"`
	Email    *string `json:"email"`
	Avatar   *string `json:"avatar"`
}

type UserService struct {
	repo userRepository
}

func NewUserService(repo userRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) GetByUUID(uuid string) (*model.User, error) {
	user, err := s.repo.GetByUUID(uuid)
	if err != nil {
		return nil, fmt.Errorf("get user by uuid in service: %w", err)
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	return user, nil
}

func (s *UserService) UpdateProfile(currentUser *model.User, targetUUID string, input UpdateProfileInput) (*model.User, error) {
	if currentUser.UUID != targetUUID && !currentUser.IsAdmin {
		return nil, ErrUserPermissionDenied
	}

	targetUser := currentUser
	if currentUser.UUID != targetUUID {
		user, err := s.repo.GetByUUID(targetUUID)
		if err != nil {
			return nil, fmt.Errorf("get target user in update profile: %w", err)
		}
		if user == nil {
			return nil, ErrUserNotFound
		}
		targetUser = user
	}

	if err := applyProfileUpdate(targetUser, input); err != nil {
		return nil, err
	}

	if err := s.repo.Update(targetUser); err != nil {
		return nil, fmt.Errorf("update user profile: %w", err)
	}

	return targetUser, nil
}

func applyProfileUpdate(user *model.User, input UpdateProfileInput) error {
	var updated bool

	if input.Nickname != nil {
		nickname := strings.TrimSpace(*input.Nickname)
		nicknameLength := utf8.RuneCountInString(nickname)
		if nicknameLength < 2 || nicknameLength > 20 {
			return ErrInvalidNickname
		}
		user.Nickname = nickname
		updated = true
	}

	if input.Email != nil {
		email := strings.ToLower(strings.TrimSpace(*input.Email))
		if email != "" {
			if len(email) > 64 {
				return ErrInvalidEmail
			}
			if _, err := mail.ParseAddress(email); err != nil {
				return ErrInvalidEmail
			}
		}
		user.Email = email
		updated = true
	}

	if input.Avatar != nil {
		avatar := strings.TrimSpace(*input.Avatar)
		if avatar == "" {
			avatar = model.DefaultAvatarURL
		}
		if len(avatar) > 255 {
			return ErrInvalidAvatar
		}
		user.Avatar = avatar
		updated = true
	}

	if !updated {
		return ErrEmptyProfileUpdate
	}

	return nil
}
