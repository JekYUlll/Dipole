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

type stubContactService struct {
	applyFn             func(currentUserUUID string, input service.ApplyContactInput) (*model.ContactApplication, error)
	listFriendsFn       func(currentUserUUID string) ([]*service.ContactListItem, error)
	listIncomingFn      func(currentUserUUID string) ([]*service.ContactApplicationView, error)
	listOutgoingFn      func(currentUserUUID string) ([]*service.ContactApplicationView, error)
	handleApplicationFn func(currentUserUUID string, applicationID uint, action string) (*model.ContactApplication, error)
	deleteFriendFn      func(currentUserUUID, friendUUID string) error
	updateRemarkFn      func(currentUserUUID, friendUUID, remark string) (*model.Contact, error)
	updateBlockStatusFn func(currentUserUUID, friendUUID string, blocked bool) (*model.Contact, error)
}

func (s *stubContactService) Apply(currentUserUUID string, input service.ApplyContactInput) (*model.ContactApplication, error) {
	return s.applyFn(currentUserUUID, input)
}
func (s *stubContactService) ListFriends(currentUserUUID string) ([]*service.ContactListItem, error) {
	return s.listFriendsFn(currentUserUUID)
}
func (s *stubContactService) ListIncomingApplications(currentUserUUID string) ([]*service.ContactApplicationView, error) {
	return s.listIncomingFn(currentUserUUID)
}
func (s *stubContactService) ListOutgoingApplications(currentUserUUID string) ([]*service.ContactApplicationView, error) {
	return s.listOutgoingFn(currentUserUUID)
}
func (s *stubContactService) HandleApplication(currentUserUUID string, applicationID uint, action string) (*model.ContactApplication, error) {
	return s.handleApplicationFn(currentUserUUID, applicationID, action)
}
func (s *stubContactService) DeleteFriend(currentUserUUID, friendUUID string) error {
	return s.deleteFriendFn(currentUserUUID, friendUUID)
}
func (s *stubContactService) UpdateRemark(currentUserUUID, friendUUID, remark string) (*model.Contact, error) {
	return s.updateRemarkFn(currentUserUUID, friendUUID, remark)
}
func (s *stubContactService) UpdateBlockStatus(currentUserUUID, friendUUID string, blocked bool) (*model.Contact, error) {
	return s.updateBlockStatusFn(currentUserUUID, friendUUID, blocked)
}

func TestContactHandlerApplySuccess(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewContactHandler(&stubContactService{
		applyFn: func(currentUserUUID string, input service.ApplyContactInput) (*model.ContactApplication, error) {
			if currentUserUUID != "U100" || input.TargetUUID != "U200" {
				t.Fatalf("unexpected apply input: %s %+v", currentUserUUID, input)
			}
			return &model.ContactApplication{ID: 1, Status: model.ContactApplicationPending}, nil
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/contacts/applications", strings.NewReader(`{"target_uuid":"U200","message":"hi"}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.Apply(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestContactHandlerApplyConflict(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewContactHandler(&stubContactService{
		applyFn: func(currentUserUUID string, input service.ApplyContactInput) (*model.ContactApplication, error) {
			return nil, service.ErrContactApplicationExists
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/contacts/applications", strings.NewReader(`{"target_uuid":"U200"}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.Apply(context)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", recorder.Code)
	}

	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if int(response["code"].(float64)) != code.ContactApplicationExists {
		t.Fatalf("expected business code %d, got %v", code.ContactApplicationExists, response["code"])
	}
}

func TestContactHandlerListApplicationsIncomingSuccess(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewContactHandler(&stubContactService{
		listIncomingFn: func(currentUserUUID string) ([]*service.ContactApplicationView, error) {
			return []*service.ContactApplicationView{}, nil
		},
		listOutgoingFn: func(currentUserUUID string) ([]*service.ContactApplicationView, error) {
			return nil, errors.New("should not be called")
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/contacts/applications?box=incoming", nil)
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.ListApplications(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestContactHandlerHandleApplicationForbidden(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewContactHandler(&stubContactService{
		handleApplicationFn: func(currentUserUUID string, applicationID uint, action string) (*model.ContactApplication, error) {
			return nil, service.ErrContactPermissionDenied
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/contacts/applications/1", strings.NewReader(`{"action":"accept"}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Params = gin.Params{{Key: "id", Value: "1"}}
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.HandleApplication(context)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", recorder.Code)
	}
}

func TestContactHandlerDeleteFriendNotFound(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewContactHandler(&stubContactService{
		deleteFriendFn: func(currentUserUUID, friendUUID string) error {
			return service.ErrContactTargetNotFound
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/contacts/U200", nil)
	context.Params = gin.Params{{Key: "friend_uuid", Value: "U200"}}
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.DeleteFriend(context)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", recorder.Code)
	}
}

func TestContactHandlerUpdateRemarkSuccess(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewContactHandler(&stubContactService{
		updateRemarkFn: func(currentUserUUID, friendUUID, remark string) (*model.Contact, error) {
			return &model.Contact{UserUUID: currentUserUUID, FriendUUID: friendUUID, Remark: remark, Status: model.ContactStatusNormal}, nil
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/contacts/U200/remark", strings.NewReader(`{"remark":"buddy"}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Params = gin.Params{{Key: "friend_uuid", Value: "U200"}}
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.UpdateRemark(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestContactHandlerUpdateBlockStatusSuccess(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewContactHandler(&stubContactService{
		updateBlockStatusFn: func(currentUserUUID, friendUUID string, blocked bool) (*model.Contact, error) {
			return &model.Contact{UserUUID: currentUserUUID, FriendUUID: friendUUID, Status: model.ContactStatusBlocked}, nil
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/contacts/U200/block", strings.NewReader(`{"blocked":true}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Params = gin.Params{{Key: "friend_uuid", Value: "U200"}}
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.UpdateBlockStatus(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}
