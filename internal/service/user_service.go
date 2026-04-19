package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/mail"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/JekYUlll/Dipole/internal/model"
	platformStorage "github.com/JekYUlll/Dipole/internal/platform/storage"
)

var (
	ErrUserNotFound             = errors.New("user not found")
	ErrUserPermissionDenied     = errors.New("user permission denied")
	ErrEmptyProfileUpdate       = errors.New("empty profile update")
	ErrInvalidNickname          = errors.New("invalid nickname")
	ErrInvalidEmail             = errors.New("invalid email")
	ErrInvalidAvatar            = errors.New("invalid avatar")
	ErrInvalidSignature         = errors.New("invalid signature")
	ErrAvatarMissing            = errors.New("avatar is missing")
	ErrAvatarTooLarge           = errors.New("avatar is too large")
	ErrAvatarStorageUnavailable = errors.New("avatar storage is unavailable")
	ErrAdminRequired            = errors.New("admin required")
	ErrInvalidUserStatus        = errors.New("invalid user status")
	ErrCannotDisableSelf        = errors.New("cannot disable self")
)

type userRepository interface {
	GetByUUID(uuid string) (*model.User, error)
	SearchActive(keyword, excludeUUID string, limit int) ([]*model.User, error)
	List(keyword string, status *int8, limit int) ([]*model.User, error)
	Update(user *model.User) error
}

type userAvatarFileRepository interface {
	Create(file *model.UploadedFile) error
	GetByUUID(uuid string) (*model.UploadedFile, error)
}

type userAvatarStorage interface {
	UploadAvatar(ctx context.Context, file multipart.File, header *multipart.FileHeader, userUUID string) (*platformStorage.UploadedObject, error)
	PresignDownloadURL(ctx context.Context, bucket, objectKey string, expiry time.Duration) (string, error)
	OpenObject(ctx context.Context, bucket, objectKey string) (io.ReadCloser, error)
}

type AvatarResponse struct {
	RedirectURL string
	ContentType string
	ContentSize int64
	Content     io.ReadCloser
	Cleanup     func()
}

type UpdateProfileInput struct {
	Nickname  *string `json:"nickname"`
	Email     *string `json:"email"`
	Avatar    *string `json:"avatar"`
	Signature *string `json:"signature"`
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
	repo           userRepository
	fileRepo       userAvatarFileRepository
	storage        userAvatarStorage
	avatarMaxBytes int64
	avatarURLTTL   time.Duration
}

func NewUserService(repo userRepository) *UserService {
	return &UserService{
		repo:           repo,
		avatarMaxBytes: 5 * 1024 * 1024,
		avatarURLTTL:   10 * time.Minute,
	}
}

func (s *UserService) WithAvatarStorage(fileRepo userAvatarFileRepository, storage userAvatarStorage, maxBytes int64, urlTTL time.Duration) *UserService {
	s.fileRepo = fileRepo
	s.storage = storage
	if maxBytes > 0 {
		s.avatarMaxBytes = maxBytes
	}
	if urlTTL > 0 {
		s.avatarURLTTL = urlTTL
	}
	return s
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
	targetUser, err := s.resolveEditableUser(currentUser, targetUUID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) || errors.Is(err, ErrUserPermissionDenied) {
			return nil, err
		}
		return nil, fmt.Errorf("resolve target user in update profile: %w", err)
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

func (s *UserService) UploadAvatar(currentUser *model.User, targetUUID string, header *multipart.FileHeader) (*model.User, error) {
	if header == nil {
		return nil, ErrAvatarMissing
	}
	if s.storage == nil || s.fileRepo == nil {
		return nil, ErrAvatarStorageUnavailable
	}
	if s.avatarMaxBytes > 0 && header.Size > s.avatarMaxBytes {
		return nil, ErrAvatarTooLarge
	}
	if !isSupportedAvatarHeader(header) {
		return nil, ErrInvalidAvatar
	}

	targetUser, err := s.resolveEditableUser(currentUser, targetUUID)
	if err != nil {
		return nil, err
	}

	file, err := header.Open()
	if err != nil {
		return nil, fmt.Errorf("open avatar file: %w", err)
	}
	defer file.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	uploaded, err := s.storage.UploadAvatar(ctx, file, header, targetUser.UUID)
	if err != nil {
		return nil, fmt.Errorf("upload avatar: %w", err)
	}

	record := &model.UploadedFile{
		UUID:         generateUploadedFileUUID(),
		UploaderUUID: targetUser.UUID,
		Bucket:       uploaded.Bucket,
		ObjectKey:    uploaded.ObjectKey,
		FileName:     uploaded.FileName,
		FileSize:     uploaded.FileSize,
		ContentType:  uploaded.ContentType,
		URL:          uploaded.URL,
	}
	if err := s.fileRepo.Create(record); err != nil {
		return nil, fmt.Errorf("persist avatar file: %w", err)
	}

	targetUser.Avatar = buildUserAvatarPath(targetUser.UUID)
	targetUser.AvatarFileUUID = record.UUID
	if err := s.repo.Update(targetUser); err != nil {
		return nil, fmt.Errorf("update user avatar: %w", err)
	}

	return targetUser, nil
}

func (s *UserService) GetAvatarResponse(targetUUID string) (*AvatarResponse, error) {
	user, err := s.repo.GetByUUID(strings.TrimSpace(targetUUID))
	if err != nil {
		return nil, fmt.Errorf("get user for avatar: %w", err)
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	if strings.TrimSpace(user.AvatarFileUUID) == "" {
		return &AvatarResponse{RedirectURL: fallbackAvatarURL(user.Avatar)}, nil
	}
	if s.storage == nil || s.fileRepo == nil {
		return nil, ErrAvatarStorageUnavailable
	}

	file, err := s.fileRepo.GetByUUID(user.AvatarFileUUID)
	if err != nil {
		return nil, fmt.Errorf("get avatar file: %w", err)
	}
	if file == nil {
		return &AvatarResponse{RedirectURL: fallbackAvatarURL(user.Avatar)}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	content, err := s.storage.OpenObject(ctx, file.Bucket, file.ObjectKey)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("open avatar object: %w", err)
	}

	contentType := strings.TrimSpace(file.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	return &AvatarResponse{
		ContentType: contentType,
		ContentSize: file.FileSize,
		Content:     content,
		Cleanup:     cancel,
	}, nil
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
		user.AvatarFileUUID = ""
		updated = true
	}

	if input.Signature != nil {
		signature := strings.TrimSpace(*input.Signature)
		if len([]rune(signature)) > 255 {
			return ErrInvalidSignature
		}
		user.Signature = signature
		updated = true
	}

	if !updated {
		return ErrEmptyProfileUpdate
	}

	return nil
}

func (s *UserService) resolveEditableUser(currentUser *model.User, targetUUID string) (*model.User, error) {
	if currentUser.UUID != targetUUID && !currentUser.IsAdmin {
		return nil, ErrUserPermissionDenied
	}

	targetUser := currentUser
	if currentUser.UUID != targetUUID {
		user, err := s.repo.GetByUUID(targetUUID)
		if err != nil {
			return nil, fmt.Errorf("get target user: %w", err)
		}
		if user == nil {
			return nil, ErrUserNotFound
		}
		targetUser = user
	}

	return targetUser, nil
}

func buildUserAvatarPath(userUUID string) string {
	return "/api/v1/users/" + strings.TrimSpace(userUUID) + "/avatar"
}

func fallbackAvatarURL(raw string) string {
	avatar := strings.TrimSpace(raw)
	if avatar == "" || strings.HasPrefix(avatar, "/api/v1/users/") {
		return model.DefaultAvatarURL
	}
	return avatar
}

func isSupportedAvatarHeader(header *multipart.FileHeader) bool {
	if header == nil {
		return false
	}

	contentType := strings.ToLower(strings.TrimSpace(header.Header.Get("Content-Type")))
	if strings.HasPrefix(contentType, "image/") {
		return true
	}

	switch strings.ToLower(filepath.Ext(header.Filename)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg":
		return true
	default:
		return false
	}
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
