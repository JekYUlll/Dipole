package gateway

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/middleware"
	"github.com/JekYUlll/Dipole/internal/model"
)

type gatewaySyncHandlerStub struct {
	userUUID string
	deviceID string
	afterSeq uint64
	limit    int
	ackedSeq uint64
}

func (s *gatewaySyncHandlerStub) List(userUUID string, afterSeq uint64, limit int) (*application.SyncPage, error) {
	s.userUUID, s.afterSeq, s.limit = userUUID, afterSeq, limit
	return &application.SyncPage{NextSeq: afterSeq}, nil
}

func (s *gatewaySyncHandlerStub) GetCheckpoint(userUUID, deviceID string) (*model.DeviceSyncCheckpoint, error) {
	s.userUUID, s.deviceID = userUUID, deviceID
	return &model.DeviceSyncCheckpoint{UserUUID: userUUID, DeviceID: deviceID}, nil
}

func (s *gatewaySyncHandlerStub) AdvanceCheckpoint(userUUID, deviceID string, syncSeq uint64) (*model.DeviceSyncCheckpoint, error) {
	s.userUUID, s.deviceID, s.ackedSeq = userUUID, deviceID, syncSeq
	return &model.DeviceSyncCheckpoint{UserUUID: userUUID, DeviceID: deviceID, SyncSeq: syncSeq}, nil
}

func (s *gatewaySyncHandlerStub) ListGroupCheckpoints(string, string, []string) ([]*model.GroupSyncCheckpoint, error) {
	return nil, nil
}

func (s *gatewaySyncHandlerStub) AdvanceGroupCheckpoint(string, string, string, uint64) (*model.GroupSyncCheckpoint, error) {
	return &model.GroupSyncCheckpoint{}, nil
}

type gatewaySyncComparisonObserver struct{ outcomes map[string]int }

func (s *gatewaySyncComparisonObserver) ObserveClientSyncComparison(outcome string, count int) {
	if s.outcomes == nil {
		s.outcomes = make(map[string]int)
	}
	s.outcomes[outcome] += count
}

func TestGatewaySyncHandlerListsAndAdvancesCheckpoint(t *testing.T) {
	stub := &gatewaySyncHandlerStub{}
	handler := NewSyncHandler(stub)

	list := newGatewaySyncContext(http.MethodGet, "/api/v1/sync?after_seq=7&limit=20", nil)
	handler.List(list)
	if list.Writer.Status() != http.StatusOK || stub.userUUID != "U100" || stub.afterSeq != 7 || stub.limit != 20 {
		t.Fatalf("sync list response=%d stub=%+v", list.Writer.Status(), stub)
	}

	checkpoint := newGatewaySyncContext(http.MethodPatch, "/api/v1/sync/checkpoint", strings.NewReader(`{"sync_seq":8}`))
	checkpoint.Request.Header.Set("Content-Type", "application/json")
	checkpoint.Request.Header.Set("X-Device-ID", "web-1")
	handler.AdvanceCheckpoint(checkpoint)
	if checkpoint.Writer.Status() != http.StatusOK || stub.deviceID != "web-1" || stub.ackedSeq != 8 {
		t.Fatalf("sync checkpoint response=%d stub=%+v", checkpoint.Writer.Status(), stub)
	}
}

func TestGatewaySyncHandlerReportsBoundedComparison(t *testing.T) {
	observer := &gatewaySyncComparisonObserver{}
	handler := NewSyncHandler(&gatewaySyncHandlerStub{}).WithComparisonObserver(observer)

	valid := newGatewaySyncContext(http.MethodPost, "/api/v1/sync/comparison", strings.NewReader(`{"baseline":true,"match":3,"timeline_missing":1}`))
	valid.Request.Header.Set("Content-Type", "application/json")
	valid.Request.Header.Set("X-Device-ID", "web-1")
	handler.ReportComparison(valid)
	if valid.Writer.Status() != http.StatusOK || observer.outcomes["baseline"] != 1 || observer.outcomes["match"] != 3 || observer.outcomes["timeline_missing"] != 1 {
		t.Fatalf("comparison response=%d outcomes=%+v", valid.Writer.Status(), observer.outcomes)
	}

	invalid := newGatewaySyncContext(http.MethodPost, "/api/v1/sync/comparison", strings.NewReader(`{"match":10001}`))
	invalid.Request.Header.Set("Content-Type", "application/json")
	invalid.Request.Header.Set("X-Device-ID", "web-1")
	handler.ReportComparison(invalid)
	if invalid.Writer.Status() != http.StatusBadRequest {
		t.Fatalf("invalid comparison response=%d", invalid.Writer.Status())
	}
}

func newGatewaySyncContext(method, target string, body io.Reader) *gin.Context {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(method, target, body)
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U100"})
	return context
}
