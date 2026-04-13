package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/JekYUlll/Dipole/internal/middleware"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/service"
)

type stubGroupService struct {
	createGroupFn   func(currentUserUUID string, input service.CreateGroupInput) (*service.GroupView, error)
	getGroupFn      func(currentUserUUID, groupUUID string) (*service.GroupView, error)
	listMembersFn   func(currentUserUUID, groupUUID string) ([]*service.GroupMemberView, error)
	addMembersFn    func(currentUserUUID, groupUUID string, memberUUIDs []string) ([]*service.GroupMemberView, error)
	leaveGroupFn    func(currentUserUUID, groupUUID string) error
	updateGroupFn   func(currentUserUUID, groupUUID string, input service.UpdateGroupInput) (*service.GroupView, error)
	removeMembersFn func(currentUserUUID, groupUUID string, memberUUIDs []string) error
	dismissGroupFn  func(currentUserUUID, groupUUID string) error
}

func (s *stubGroupService) CreateGroup(currentUserUUID string, input service.CreateGroupInput) (*service.GroupView, error) {
	return s.createGroupFn(currentUserUUID, input)
}

func (s *stubGroupService) GetGroup(currentUserUUID, groupUUID string) (*service.GroupView, error) {
	return s.getGroupFn(currentUserUUID, groupUUID)
}

func (s *stubGroupService) ListMembers(currentUserUUID, groupUUID string) ([]*service.GroupMemberView, error) {
	return s.listMembersFn(currentUserUUID, groupUUID)
}

func (s *stubGroupService) AddMembers(currentUserUUID, groupUUID string, memberUUIDs []string) ([]*service.GroupMemberView, error) {
	return s.addMembersFn(currentUserUUID, groupUUID, memberUUIDs)
}

func (s *stubGroupService) LeaveGroup(currentUserUUID, groupUUID string) error {
	return s.leaveGroupFn(currentUserUUID, groupUUID)
}

func (s *stubGroupService) UpdateGroup(currentUserUUID, groupUUID string, input service.UpdateGroupInput) (*service.GroupView, error) {
	return s.updateGroupFn(currentUserUUID, groupUUID, input)
}

func (s *stubGroupService) RemoveMembers(currentUserUUID, groupUUID string, memberUUIDs []string) error {
	return s.removeMembersFn(currentUserUUID, groupUUID, memberUUIDs)
}

func (s *stubGroupService) DismissGroup(currentUserUUID, groupUUID string) error {
	return s.dismissGroupFn(currentUserUUID, groupUUID)
}

func TestGroupHandlerCreateSuccess(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewGroupHandler(&stubGroupService{
		createGroupFn: func(currentUserUUID string, input service.CreateGroupInput) (*service.GroupView, error) {
			return &service.GroupView{
				Group: &model.Group{UUID: "G100", Name: input.Name, MemberCount: 1},
				Owner: &model.User{UUID: currentUserUUID, Nickname: "owner"},
			}, nil
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/groups", strings.NewReader(`{"name":"Team"}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.Create(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestGroupHandlerGetForbidden(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewGroupHandler(&stubGroupService{
		getGroupFn: func(currentUserUUID, groupUUID string) (*service.GroupView, error) {
			return nil, service.ErrGroupPermissionDenied
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/groups/G100", nil)
	context.Params = gin.Params{{Key: "uuid", Value: "G100"}}
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.Get(context)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", recorder.Code)
	}
}

func TestGroupHandlerAddMembersSuccess(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewGroupHandler(&stubGroupService{
		addMembersFn: func(currentUserUUID, groupUUID string, memberUUIDs []string) ([]*service.GroupMemberView, error) {
			return []*service.GroupMemberView{}, nil
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/groups/G100/members", strings.NewReader(`{"member_uuids":["U200"]}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Params = gin.Params{{Key: "uuid", Value: "G100"}}
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.AddMembers(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestGroupHandlerLeaveConflict(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewGroupHandler(&stubGroupService{
		leaveGroupFn: func(currentUserUUID, groupUUID string) error {
			return service.ErrGroupOwnerCannotLeave
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/groups/G100/members/me", nil)
	context.Params = gin.Params{{Key: "uuid", Value: "G100"}}
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.Leave(context)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", recorder.Code)
	}
}

func TestGroupHandlerUpdateSuccess(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewGroupHandler(&stubGroupService{
		updateGroupFn: func(currentUserUUID, groupUUID string, input service.UpdateGroupInput) (*service.GroupView, error) {
			return &service.GroupView{
				Group: &model.Group{UUID: groupUUID, Name: input.Name},
				Owner: &model.User{UUID: currentUserUUID},
			}, nil
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/groups/G100/update", strings.NewReader(`{"name":"New Team"}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Params = gin.Params{{Key: "uuid", Value: "G100"}}
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.Update(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestGroupHandlerRemoveMembersConflict(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewGroupHandler(&stubGroupService{
		removeMembersFn: func(currentUserUUID, groupUUID string, memberUUIDs []string) error {
			return service.ErrGroupOwnerCannotBeRemoved
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/groups/G100/remove-members", strings.NewReader(`{"member_uuids":["U100"]}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Params = gin.Params{{Key: "uuid", Value: "G100"}}
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.RemoveMembers(context)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", recorder.Code)
	}
}

func TestGroupHandlerDismissSuccess(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewGroupHandler(&stubGroupService{
		dismissGroupFn: func(currentUserUUID, groupUUID string) error {
			return nil
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/groups/G100/dismiss", nil)
	context.Params = gin.Params{{Key: "uuid", Value: "G100"}}
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})

	handler.Dismiss(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}
