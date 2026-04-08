package http

import (
	"encoding/json"
	"errors"
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
	getByUUIDFn     func(uuid string) (*model.User, error)
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
