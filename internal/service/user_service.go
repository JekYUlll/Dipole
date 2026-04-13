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
	ErrAdminRequired        = errors.New("admin required")
	ErrInvalidUserStatus    = errors.New("invalid user status")
	ErrCannotDisableSelf    = errors.New("cannot disable self")
)

type userRepository interface {
	GetByUUID(uuid string) (*model.User, error)
	SearchActive(keyword, excludeUUID string, limit int) ([]*model.User, error)
	List(keyword string, status *int8, limit int) ([]*model.User, error)
	Update(user *model.User) error
}

type UpdateProfileInput struct {
	Nickname *string `json:"nickname"`
	Email    *string `json:"email"`
	Avatar   *string `json:"avatar"`
}

type SearchUsersInput struct {
	Keyword string
	Limit   int
}

type AdminListUsersInput struct {
	Keyword string
	Status  *int8
	Limit   int
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

func (s *UserService) SearchUsers(currentUser *model.User, input SearchUsersInput) ([]*model.User, error) {
	users, err := s.repo.SearchActive(strings.TrimSpace(input.Keyword), currentUser.UUID, normalizeUserListLimit(input.Limit))
	if err != nil {
		return nil, fmt.Errorf("search users: %w", err)
	}

	return users, nil
}

func (s *UserService) ListUsersForAdmin(currentUser *model.User, input AdminListUsersInput) ([]*model.User, error) {
	if !currentUser.IsAdmin {
		return nil, ErrAdminRequired
	}

	if input.Status != nil && !isValidUserStatus(*input.Status) {
		return nil, ErrInvalidUserStatus
	}

	users, err := s.repo.List(strings.TrimSpace(input.Keyword), input.Status, normalizeUserListLimit(input.Limit))
	if err != nil {
		return nil, fmt.Errorf("list users for admin: %w", err)
	}

	return users, nil
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

func (s *UserService) UpdateStatus(currentUser *model.User, targetUUID string, status int8) (*model.User, error) {
	if !currentUser.IsAdmin {
		return nil, ErrAdminRequired
	}
	if !isValidUserStatus(status) {
		return nil, ErrInvalidUserStatus
	}
	if currentUser.UUID == targetUUID && status == model.UserStatusDisabled {
		return nil, ErrCannotDisableSelf
	}

	user, err := s.repo.GetByUUID(targetUUID)
	if err != nil {
		return nil, fmt.Errorf("get user in update status: %w", err)
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	user.Status = status
	if err := s.repo.Update(user); err != nil {
		return nil, fmt.Errorf("update user status: %w", err)
	}

	return user, nil
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

func normalizeUserListLimit(limit int) int {
	switch {
	case limit <= 0:
		return 20
	case limit > 50:
		return 50
	default:
		return limit
	}
}

func isValidUserStatus(status int8) bool {
	return status == model.UserStatusNormal || status == model.UserStatusDisabled
}
