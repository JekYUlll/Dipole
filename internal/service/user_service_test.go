package service

import (
	"context"
	"errors"
	"io"
	"mime/multipart"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
	platformStorage "github.com/JekYUlll/Dipole/internal/platform/storage"
)

type stubUserRepository struct {
	users       map[string]*model.User
	updateCalls int
	updateErr   error
	searchUsers []*model.User
	listUsers   []*model.User
}

type stubUserAvatarFileRepository struct {
	files     map[string]*model.UploadedFile
	createErr error
}

func (r *stubUserAvatarFileRepository) Create(file *model.UploadedFile) error {
	if r.createErr != nil {
		return r.createErr
	}
	if r.files == nil {
		r.files = map[string]*model.UploadedFile{}
	}
	r.files[file.UUID] = file
	return nil
}

func (r *stubUserAvatarFileRepository) GetByUUID(uuid string) (*model.UploadedFile, error) {
	if r.files == nil {
		return nil, nil
	}
	return r.files[uuid], nil
}

type stubUserAvatarStorage struct {
	uploadObject *platformStorage.UploadedObject
	uploadErr    error
	presignedURL string
	presignErr   error
	objectBody   string
	openErr      error
}

func (s *stubUserAvatarStorage) UploadAvatar(ctx context.Context, file multipart.File, header *multipart.FileHeader, userUUID string) (*platformStorage.UploadedObject, error) {
	_ = ctx
	_ = file
	_ = header
	_ = userUUID
	if s.uploadErr != nil {
		return nil, s.uploadErr
	}
	if s.uploadObject != nil {
		return s.uploadObject, nil
	}
	return &platformStorage.UploadedObject{
		Bucket:      "dipole-files",
		ObjectKey:   "avatars/U100.png",
		FileName:    "avatar.png",
		FileSize:    12,
		ContentType: "image/png",
		URL:         "http://example.com/avatar.png",
	}, nil
}

func (s *stubUserAvatarStorage) PresignDownloadURL(ctx context.Context, bucket, objectKey string, expiry time.Duration) (string, error) {
	_ = ctx
	_ = bucket
	_ = objectKey
	_ = expiry
	if s.presignErr != nil {
		return "", s.presignErr
	}
	if s.presignedURL != "" {
		return s.presignedURL, nil
	}
	return "https://example.com/avatar.png", nil
}

func (s *stubUserAvatarStorage) OpenObject(ctx context.Context, bucket, objectKey string) (io.ReadCloser, error) {
	_ = ctx
	_ = bucket
	_ = objectKey
	if s.openErr != nil {
		return nil, s.openErr
	}
	body := s.objectBody
	if body == "" {
		body = "avatar-bytes"
	}
	return io.NopCloser(strings.NewReader(body)), nil
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

func (r *stubUserRepository) SearchActive(keyword, excludeUUID string, limit int) ([]*model.User, error) {
	if r.searchUsers != nil {
		return r.searchUsers, nil
	}

	var users []*model.User
	for _, user := range r.users {
		if user.UUID == excludeUUID || user.Status != model.UserStatusNormal {
			continue
		}
		users = append(users, user)
	}

	return users, nil
}

func (r *stubUserRepository) List(keyword string, status *int8, limit int) ([]*model.User, error) {
	if r.listUsers != nil {
		return r.listUsers, nil
	}

	var users []*model.User
	for _, user := range r.users {
		if status != nil && user.Status != *status {
			continue
		}
		users = append(users, user)
	}

	return users, nil
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
	signature := "hello dipole"

	updatedUser, err := service.UpdateProfile(currentUser, "U100", UpdateProfileInput{
		Nickname:  &nickname,
		Email:     &email,
		Avatar:    &avatar,
		Signature: &signature,
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
	if updatedUser.Signature != signature {
		t.Fatalf("expected signature updated, got %s", updatedUser.Signature)
	}
	if repo.updateCalls != 1 {
		t.Fatalf("expected one update call, got %d", repo.updateCalls)
	}
}

func TestUserServiceUpdateProfileRejectsTooLongSignature(t *testing.T) {
	t.Parallel()

	repo := &stubUserRepository{
		users: map[string]*model.User{
			"U100": {UUID: "U100", Nickname: "Owner"},
		},
	}
	service := NewUserService(repo)
	signature := strings.Repeat("a", 256)

	_, err := service.UpdateProfile(repo.users["U100"], "U100", UpdateProfileInput{
		Signature: &signature,
	})
	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("expected ErrInvalidSignature, got %v", err)
	}
}

func TestUserServiceUploadAvatarSuccess(t *testing.T) {
	t.Parallel()

	repo := &stubUserRepository{
		users: map[string]*model.User{
			"U100": {UUID: "U100", Nickname: "Alice", Avatar: model.DefaultAvatarURL},
		},
	}
	fileRepo := &stubUserAvatarFileRepository{}
	storage := &stubUserAvatarStorage{}
	service := NewUserService(repo).WithAvatarStorage(fileRepo, storage, 1024, time.Minute)

	header := newTestFileHeader(t, "avatar.png", "image/png", []byte("avatar-bytes"))

	updatedUser, err := service.UploadAvatar(repo.users["U100"], "U100", header)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if updatedUser.Avatar != "/api/v1/users/U100/avatar" {
		t.Fatalf("unexpected avatar path: %s", updatedUser.Avatar)
	}
	if updatedUser.AvatarFileUUID == "" {
		t.Fatalf("expected avatar file uuid to be set")
	}
	if repo.updateCalls != 1 {
		t.Fatalf("expected one update call, got %d", repo.updateCalls)
	}
}

func TestUserServiceGetAvatarResponseFallsBackToDefault(t *testing.T) {
	t.Parallel()

	repo := &stubUserRepository{
		users: map[string]*model.User{
			"U100": {UUID: "U100", Nickname: "Alice", Avatar: "/api/v1/users/U100/avatar"},
		},
	}
	service := NewUserService(repo)

	avatar, err := service.GetAvatarResponse("U100")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if avatar == nil || avatar.RedirectURL != model.DefaultAvatarURL {
		t.Fatalf("expected default avatar url, got %+v", avatar)
	}
}

func TestUserServiceGetAvatarResponseWithStoredFile(t *testing.T) {
	t.Parallel()

	repo := &stubUserRepository{
		users: map[string]*model.User{
			"U100": {UUID: "U100", Nickname: "Alice", Avatar: "/api/v1/users/U100/avatar", AvatarFileUUID: "F100"},
		},
	}
	fileRepo := &stubUserAvatarFileRepository{
		files: map[string]*model.UploadedFile{
			"F100": {UUID: "F100", Bucket: "dipole-files", ObjectKey: "avatars/U100.png", FileSize: 12, ContentType: "image/png"},
		},
	}
	storage := &stubUserAvatarStorage{objectBody: "avatar-payload"}
	service := NewUserService(repo).WithAvatarStorage(fileRepo, storage, 1024, time.Minute)

	avatar, err := service.GetAvatarResponse("U100")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if avatar == nil || avatar.Content == nil {
		t.Fatalf("expected avatar content stream, got %+v", avatar)
	}
	defer avatar.Content.Close()
	data, err := io.ReadAll(avatar.Content)
	if err != nil {
		t.Fatalf("read avatar content: %v", err)
	}
	if string(data) != "avatar-payload" {
		t.Fatalf("unexpected avatar content: %s", string(data))
	}
	if avatar.ContentType != "image/png" {
		t.Fatalf("unexpected avatar content type: %s", avatar.ContentType)
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

func TestUserServiceUploadAvatarRejectsInvalidType(t *testing.T) {
	t.Parallel()

	repo := &stubUserRepository{
		users: map[string]*model.User{
			"U100": {UUID: "U100", Nickname: "Alice"},
		},
	}
	service := NewUserService(repo).WithAvatarStorage(&stubUserAvatarFileRepository{}, &stubUserAvatarStorage{}, 1024, time.Minute)

	header := newTestFileHeader(t, "avatar.txt", "text/plain", []byte("avatar-bytes"))

	_, err := service.UploadAvatar(repo.users["U100"], "U100", header)
	if !errors.Is(err, ErrInvalidAvatar) {
		t.Fatalf("expected ErrInvalidAvatar, got %v", err)
	}
}

func TestUserServiceUploadAvatarRejectsTooLarge(t *testing.T) {
	t.Parallel()

	repo := &stubUserRepository{
		users: map[string]*model.User{
			"U100": {UUID: "U100", Nickname: "Alice"},
		},
	}
	service := NewUserService(repo).WithAvatarStorage(&stubUserAvatarFileRepository{}, &stubUserAvatarStorage{}, 8, time.Minute)

	header := newTestFileHeader(t, "avatar.png", "image/png", []byte("avatar-bytes"))

	_, err := service.UploadAvatar(repo.users["U100"], "U100", header)
	if !errors.Is(err, ErrAvatarTooLarge) {
		t.Fatalf("expected ErrAvatarTooLarge, got %v", err)
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

func TestUserServiceSearchUsersExcludesCurrentUser(t *testing.T) {
	t.Parallel()

	repo := &stubUserRepository{
		searchUsers: []*model.User{
			{UUID: "U200", Nickname: "Alice", Status: model.UserStatusNormal},
			{UUID: "U300", Nickname: "Bob", Status: model.UserStatusNormal},
		},
	}
	service := NewUserService(repo)
	currentUser := &model.User{UUID: "U100"}

	users, err := service.SearchUsers(currentUser, SearchUsersInput{
		Keyword: "A",
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
}

func TestUserServiceListUsersForAdminRequiresAdmin(t *testing.T) {
	t.Parallel()

	service := NewUserService(&stubUserRepository{})
	currentUser := &model.User{UUID: "U100", IsAdmin: false}

	_, err := service.ListUsersForAdmin(currentUser, AdminListUsersInput{})
	if !errors.Is(err, ErrAdminRequired) {
		t.Fatalf("expected ErrAdminRequired, got %v", err)
	}
}

func TestUserServiceUpdateStatusSuccess(t *testing.T) {
	t.Parallel()

	repo := &stubUserRepository{
		users: map[string]*model.User{
			"U200": {UUID: "U200", Status: model.UserStatusNormal},
		},
	}
	service := NewUserService(repo)
	admin := &model.User{UUID: "U999", IsAdmin: true}

	user, err := service.UpdateStatus(admin, "U200", model.UserStatusDisabled)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if user.Status != model.UserStatusDisabled {
		t.Fatalf("expected status %d, got %d", model.UserStatusDisabled, user.Status)
	}
	if repo.updateCalls != 1 {
		t.Fatalf("expected one update call, got %d", repo.updateCalls)
	}
}

func TestUserServiceUpdateStatusRejectsSelfDisable(t *testing.T) {
	t.Parallel()

	service := NewUserService(&stubUserRepository{
		users: map[string]*model.User{
			"U999": {UUID: "U999", Status: model.UserStatusNormal},
		},
	})
	admin := &model.User{UUID: "U999", IsAdmin: true}

	_, err := service.UpdateStatus(admin, "U999", model.UserStatusDisabled)
	if !errors.Is(err, ErrCannotDisableSelf) {
		t.Fatalf("expected ErrCannotDisableSelf, got %v", err)
	}
}
