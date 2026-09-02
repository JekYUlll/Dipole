package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/JekYUlll/Dipole/internal/code"
	"github.com/JekYUlll/Dipole/internal/middleware"
	"github.com/JekYUlll/Dipole/internal/model"
	coreadmin "github.com/JekYUlll/Dipole/internal/services/core/domain/admin"
)

type stubAdminService struct {
	overviewFn func(currentUser *model.User) (*coreadmin.AdminOverview, error)
}

func (s *stubAdminService) Overview(currentUser *model.User) (*coreadmin.AdminOverview, error) {
	if s.overviewFn == nil {
		return nil, nil
	}
	return s.overviewFn(currentUser)
}

func TestAdminHandlerOverviewSuccess(t *testing.T) {
	t.Parallel()

	handler := NewAdminHandler(&stubAdminService{
		overviewFn: func(currentUser *model.User) (*coreadmin.AdminOverview, error) {
			if currentUser.UUID != "U100" || !currentUser.IsAdmin {
				t.Fatalf("unexpected current user: %+v", currentUser)
			}
			return &coreadmin.AdminOverview{
				AppName:               "dipole",
				Env:                   "local",
				UserTotal:             10,
				MessageTotal:          20,
				OnlineUserTotal:       3,
				OnlineConnectionTotal: 4,
				KafkaEnabled:          true,
				TLSEnabled:            true,
			}, nil
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/overview", nil)
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100", IsAdmin: true})

	handler.Overview(context)

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

func TestAdminHandlerOverviewRequiresAdmin(t *testing.T) {
	t.Parallel()

	handler := NewAdminHandler(&stubAdminService{
		overviewFn: func(currentUser *model.User) (*coreadmin.AdminOverview, error) {
			return nil, coreadmin.ErrAdminRequired
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/overview", nil)
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100", IsAdmin: false})

	handler.Overview(context)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", recorder.Code)
	}
}

func TestAdminHandlerOverviewHandlesInternalError(t *testing.T) {
	t.Parallel()

	handler := NewAdminHandler(&stubAdminService{
		overviewFn: func(currentUser *model.User) (*coreadmin.AdminOverview, error) {
			return nil, errors.New("boom")
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/overview", nil)
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100", IsAdmin: true})

	handler.Overview(context)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", recorder.Code)
	}
}
