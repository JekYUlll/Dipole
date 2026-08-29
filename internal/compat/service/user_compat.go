package service

import (
	"github.com/JekYUlll/Dipole/internal/model"
	coreuser "github.com/JekYUlll/Dipole/internal/services/core/domain/user"
)

var (
	ErrUserNotFound             = coreuser.ErrUserNotFound
	ErrUserPermissionDenied     = coreuser.ErrUserPermissionDenied
	ErrEmptyProfileUpdate       = coreuser.ErrEmptyProfileUpdate
	ErrInvalidNickname          = coreuser.ErrInvalidNickname
	ErrInvalidEmail             = coreuser.ErrInvalidEmail
	ErrInvalidAvatar            = coreuser.ErrInvalidAvatar
	ErrInvalidSignature         = coreuser.ErrInvalidSignature
	ErrAvatarMissing            = coreuser.ErrAvatarMissing
	ErrAvatarTooLarge           = coreuser.ErrAvatarTooLarge
	ErrAvatarStorageUnavailable = coreuser.ErrAvatarStorageUnavailable
	ErrInvalidUserStatus        = coreuser.ErrInvalidUserStatus
	ErrCannotDisableSelf        = coreuser.ErrCannotDisableSelf
)

type AvatarResponse = coreuser.AvatarResponse
type UpdateProfileInput = coreuser.UpdateProfileInput
type SearchUsersInput = coreuser.SearchUsersInput
type AdminListUsersInput = coreuser.AdminListUsersInput
type UserService = coreuser.UserService

func NewUserService(repo interface {
	GetByUUID(uuid string) (*model.User, error)
	SearchActive(keyword, excludeUUID string, limit int) ([]*model.User, error)
	List(keyword string, status *int8, limit int) ([]*model.User, error)
	Update(user *model.User) error
}) *UserService {
	return coreuser.NewUserService(repo)
}
