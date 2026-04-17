package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/JekYUlll/Dipole/internal/code"
	"github.com/JekYUlll/Dipole/internal/middleware"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/service"
)

type stubUserService struct {
	updateProfileFn func(currentUser *model.User, targetUUID string, input service.UpdateProfileInput) (*model.User, error)
	getAvatarFn     func(targetUUID string) (*service.AvatarResponse, error)
	uploadAvatarFn  func(currentUser *model.User, targetUUID string, header *multipart.FileHeader) (*model.User, error)
	getByUUIDFn     func(uuid string) (*model.User, error)
	searchUsersFn   func(currentUser *model.User, input service.SearchUsersInput) ([]*model.User, error)
	listUsersFn     func(currentUser *model.User, input service.AdminListUsersInput) ([]*model.User, error)
	updateStatusFn  func(currentUser *model.User, targetUUID string, status int8) (*model.User, error)
}

func (s *stubUserService) GetByUUID(uuid string) (*model.User, error) {
	if s.getByUUIDFn == nil {
		return nil, nil
	}

	return s.getByUUIDFn(uuid)
}

func (s *stubUserService) UpdateProfile(currentUser *model.User, targetUUID string, input service.UpdateProfileInput) (*model.User, error) {
	if s.updateProfileFn == nil {
		return nil, nil
	}

	return s.updateProfileFn(currentUser, targetUUID, input)
}

func (s *stubUserService) GetAvatarResponse(targetUUID string) (*service.AvatarResponse, error) {
	if s.getAvatarFn == nil {
		return nil, nil
	}

	return s.getAvatarFn(targetUUID)
}

func (s *stubUserService) UploadAvatar(currentUser *model.User, targetUUID string, header *multipart.FileHeader) (*model.User, error) {
	if s.uploadAvatarFn == nil {
		return nil, nil
	}

	return s.uploadAvatarFn(currentUser, targetUUID, header)
}

func (s *stubUserService) SearchUsers(currentUser *model.User, input service.SearchUsersInput) ([]*model.User, error) {
	if s.searchUsersFn == nil {
		return nil, nil
	}

	return s.searchUsersFn(currentUser, input)
}

func (s *stubUserService) ListUsersForAdmin(currentUser *model.User, input service.AdminListUsersInput) ([]*model.User, error) {
	if s.listUsersFn == nil {
		return nil, nil
	}

	return s.listUsersFn(currentUser, input)
}

func (s *stubUserService) UpdateStatus(currentUser *model.User, targetUUID string, status int8) (*model.User, error) {
	if s.updateStatusFn == nil {
		return nil, nil
	}

	return s.updateStatusFn(currentUser, targetUUID, status)
}

func TestUserHandlerUpdateProfileSuccess(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewUserHandler(&stubUserService{
		updateProfileFn: func(currentUser *model.User, targetUUID string, input service.UpdateProfileInput) (*model.User, error) {
			if currentUser.UUID != "U100" {
				t.Fatalf("unexpected current user: %s", currentUser.UUID)
			}
			if targetUUID != "U100" {
				t.Fatalf("unexpected target uuid: %s", targetUUID)
			}
			if input.Nickname == nil || *input.Nickname != "NewName" {
				t.Fatalf("unexpected nickname input: %+v", input)
			}

			return &model.User{
				UUID:     "U100",
				Nickname: "NewName",
			}, nil
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/users/U100/profile", strings.NewReader(`{"nickname":"NewName"}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Params = gin.Params{{Key: "uuid", Value: "U100"}}
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.UpdateProfile(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if int(response["code"].(float64)) != code.Success {
		t.Fatalf("expected business code %d, got %v", code.Success, response["code"])
	}
}

func TestUserHandlerUpdateProfileForbidden(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewUserHandler(&stubUserService{
		updateProfileFn: func(currentUser *model.User, targetUUID string, input service.UpdateProfileInput) (*model.User, error) {
			return nil, service.ErrUserPermissionDenied
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/users/U200/profile", strings.NewReader(`{"nickname":"NewName"}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Params = gin.Params{{Key: "uuid", Value: "U200"}}
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.UpdateProfile(context)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", recorder.Code)
	}

	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if int(response["code"].(float64)) != code.UserPermissionDenied {
		t.Fatalf("expected business code %d, got %v", code.UserPermissionDenied, response["code"])
	}
}

func TestUserHandlerGetByUUIDNotFound(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewUserHandler(&stubUserService{
		getByUUIDFn: func(uuid string) (*model.User, error) {
			return nil, service.ErrUserNotFound
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/users/U404", nil)
	context.Params = gin.Params{{Key: "uuid", Value: "U404"}}

	handler.GetByUUID(context)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", recorder.Code)
	}

	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if int(response["code"].(float64)) != code.UserNotFound {
		t.Fatalf("expected business code %d, got %v", code.UserNotFound, response["code"])
	}
}

func TestUserHandlerUpdateProfileBadRequest(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewUserHandler(&stubUserService{
		updateProfileFn: func(currentUser *model.User, targetUUID string, input service.UpdateProfileInput) (*model.User, error) {
			return nil, errors.New("should not be called")
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/users/U100/profile", strings.NewReader(`{`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Params = gin.Params{{Key: "uuid", Value: "U100"}}
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.UpdateProfile(context)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}

	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if int(response["code"].(float64)) != code.BadRequest {
		t.Fatalf("expected business code %d, got %v", code.BadRequest, response["code"])
	}
}

func TestUserHandlerUploadAvatarSuccess(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewUserHandler(&stubUserService{
		uploadAvatarFn: func(currentUser *model.User, targetUUID string, header *multipart.FileHeader) (*model.User, error) {
			if currentUser.UUID != "U100" || targetUUID != "U100" {
				t.Fatalf("unexpected upload target: current=%s target=%s", currentUser.UUID, targetUUID)
			}
			if header == nil || header.Filename != "avatar.png" {
				t.Fatalf("unexpected file header: %+v", header)
			}
			return &model.User{UUID: "U100", Nickname: "Alice", Avatar: "/api/v1/users/U100/avatar"}, nil
		},
	})

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("avatar", "avatar.png")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte("avatar-bytes")); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/users/U100/avatar", &body)
	context.Request.Header.Set("Content-Type", writer.FormDataContentType())
	context.Params = gin.Params{{Key: "uuid", Value: "U100"}}
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.UploadAvatar(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestUserHandlerGetAvatarRedirects(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewUserHandler(&stubUserService{
		getAvatarFn: func(targetUUID string) (*service.AvatarResponse, error) {
			if targetUUID != "U100" {
				t.Fatalf("unexpected target uuid: %s", targetUUID)
			}
			return &service.AvatarResponse{RedirectURL: "https://example.com/avatar.png"}, nil
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/users/U100/avatar", nil)
	context.Params = gin.Params{{Key: "uuid", Value: "U100"}}

	handler.GetAvatar(context)

	if recorder.Code != http.StatusFound {
		t.Fatalf("expected status 302, got %d", recorder.Code)
	}
	if location := recorder.Header().Get("Location"); location != "https://example.com/avatar.png" {
		t.Fatalf("unexpected redirect location: %s", location)
	}
}

func TestUserHandlerGetAvatarStreamsContent(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewUserHandler(&stubUserService{
		getAvatarFn: func(targetUUID string) (*service.AvatarResponse, error) {
			if targetUUID != "U100" {
				t.Fatalf("unexpected target uuid: %s", targetUUID)
			}
			return &service.AvatarResponse{
				ContentType: "image/png",
				ContentSize: 6,
				Content:     io.NopCloser(strings.NewReader("avatar")),
			}, nil
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/users/U100/avatar", nil)
	context.Params = gin.Params{{Key: "uuid", Value: "U100"}}

	handler.GetAvatar(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if body := recorder.Body.String(); body != "avatar" {
		t.Fatalf("unexpected body: %s", body)
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "image/png" {
		t.Fatalf("unexpected content type: %s", contentType)
	}
}

func TestUserHandlerSearchSuccess(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewUserHandler(&stubUserService{
		searchUsersFn: func(currentUser *model.User, input service.SearchUsersInput) ([]*model.User, error) {
			if currentUser.UUID != "U100" {
				t.Fatalf("unexpected current user: %s", currentUser.UUID)
			}
			if input.Keyword != "alice" {
				t.Fatalf("unexpected keyword: %s", input.Keyword)
			}
			return []*model.User{
				{UUID: "U200", Nickname: "Alice", Avatar: "avatar", Status: model.UserStatusNormal},
			}, nil
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/users?keyword=alice", nil)
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.Search(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestUserHandlerListForAdminForbidden(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewUserHandler(&stubUserService{
		listUsersFn: func(currentUser *model.User, input service.AdminListUsersInput) ([]*model.User, error) {
			return nil, service.ErrAdminRequired
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.ListForAdmin(context)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", recorder.Code)
	}
}

func TestUserHandlerUpdateStatusRejectsSelfDisable(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewUserHandler(&stubUserService{
		updateStatusFn: func(currentUser *model.User, targetUUID string, status int8) (*model.User, error) {
			return nil, service.ErrCannotDisableSelf
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/U100/status", strings.NewReader(`{"status":1}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Params = gin.Params{{Key: "uuid", Value: "U100"}}
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100", IsAdmin: true})

	handler.UpdateStatus(context)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}

	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if int(response["code"].(float64)) != code.UserSelfStatusChange {
		t.Fatalf("expected business code %d, got %v", code.UserSelfStatusChange, response["code"])
	}
}
