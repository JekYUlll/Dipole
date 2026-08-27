package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	applicationPort "github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/middleware"
	"github.com/JekYUlll/Dipole/internal/model"
)

type stubSyncService struct {
	userUUID      string
	afterSeq      uint64
	limit         int
	deviceID      string
	checkpointSeq uint64
	groupUUIDs    []string
	groupUUID     string
}

type syncComparisonObserverStub struct {
	outcomes map[string]int
}

func (s *syncComparisonObserverStub) ObserveClientSyncComparison(outcome string, count int) {
	if s.outcomes == nil {
		s.outcomes = make(map[string]int)
	}
	s.outcomes[outcome] += count
}

func TestSyncHandlerRecordsBoundedClientComparison(t *testing.T) {
	observer := &syncComparisonObserverStub{}
	handler := NewSyncHandler(&stubSyncService{}).WithComparisonObserver(observer)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/sync/comparison", strings.NewReader(`{"baseline":false,"match":3,"pending":2,"legacy_only":1,"sync_only":0,"overflow":0,"storage_full":1,"sync_error":2,"timeline_match":4,"timeline_missing":1}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Request.Header.Set("X-Device-ID", "web-1")
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U1"})

	handler.ReportComparison(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if observer.outcomes["match"] != 3 || observer.outcomes["pending"] != 2 || observer.outcomes["legacy_only"] != 1 || observer.outcomes["storage_full"] != 1 || observer.outcomes["sync_error"] != 2 || observer.outcomes["timeline_match"] != 4 || observer.outcomes["timeline_missing"] != 1 {
		t.Fatalf("unexpected outcomes: %+v", observer.outcomes)
	}
}

func TestSyncHandlerRejectsInvalidClientComparison(t *testing.T) {
	handler := NewSyncHandler(&stubSyncService{}).WithComparisonObserver(&syncComparisonObserverStub{})
	for name, testCase := range map[string]struct {
		deviceID string
		body     string
	}{
		"missing device":           {body: `{"match":1}`},
		"negative count":           {deviceID: "web-1", body: `{"match":-1}`},
		"excessive count":          {deviceID: "web-1", body: `{"pending":10001}`},
		"excessive error count":    {deviceID: "web-1", body: `{"storage_full":10001}`},
		"excessive timeline count": {deviceID: "web-1", body: `{"timeline_mismatch":10001}`},
	} {
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/sync/comparison", strings.NewReader(testCase.body))
			context.Request.Header.Set("Content-Type", "application/json")
			context.Request.Header.Set("X-Device-ID", testCase.deviceID)
			context.Set(middleware.ContextUserKey, &model.User{UUID: "U1"})

			handler.ReportComparison(context)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400, got %d: %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func (s *stubSyncService) GetCheckpoint(userUUID, deviceID string) (*model.DeviceSyncCheckpoint, error) {
	s.userUUID = userUUID
	s.deviceID = deviceID
	return &model.DeviceSyncCheckpoint{UserUUID: userUUID, DeviceID: deviceID}, nil
}

func (s *stubSyncService) AdvanceCheckpoint(userUUID, deviceID string, syncSeq uint64) (*model.DeviceSyncCheckpoint, error) {
	s.userUUID = userUUID
	s.deviceID = deviceID
	s.checkpointSeq = syncSeq
	return &model.DeviceSyncCheckpoint{UserUUID: userUUID, DeviceID: deviceID, SyncSeq: syncSeq}, nil
}

func (s *stubSyncService) ListGroupCheckpoints(userUUID, deviceID string, groupUUIDs []string) ([]*model.GroupSyncCheckpoint, error) {
	s.userUUID, s.deviceID, s.groupUUIDs = userUUID, deviceID, groupUUIDs
	return []*model.GroupSyncCheckpoint{{GroupUUID: groupUUIDs[0], LatestMessageSeq: 10, PulledMessageSeq: 7}}, nil
}

func (s *stubSyncService) AdvanceGroupCheckpoint(userUUID, deviceID, groupUUID string, messageSeq uint64) (*model.GroupSyncCheckpoint, error) {
	s.userUUID, s.deviceID, s.groupUUID, s.checkpointSeq = userUUID, deviceID, groupUUID, messageSeq
	return &model.GroupSyncCheckpoint{GroupUUID: groupUUID, LatestMessageSeq: 10, PulledMessageSeq: messageSeq}, nil
}

func TestSyncHandlerAdvancesDeviceCheckpoint(t *testing.T) {
	stub := &stubSyncService{}
	handler := NewSyncHandler(stub)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/sync/checkpoint", strings.NewReader(`{"sync_seq":8}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Request.Header.Set("X-Device-ID", "web-1")
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U200"})

	handler.AdvanceCheckpoint(context)

	if recorder.Code != http.StatusOK || stub.userUUID != "U200" || stub.deviceID != "web-1" || stub.checkpointSeq != 8 {
		t.Fatalf("unexpected checkpoint request: status=%d stub=%+v body=%s", recorder.Code, stub, recorder.Body.String())
	}
}

func (s *stubSyncService) List(userUUID string, afterSeq uint64, limit int) (*applicationPort.SyncPage, error) {
	s.userUUID = userUUID
	s.afterSeq = afterSeq
	s.limit = limit
	return &applicationPort.SyncPage{
		Items: []*model.SyncMessage{{
			SyncSeq:         8,
			ConversationKey: "direct:U100:U200",
			MessageUUID:     "M8",
			MessageSeq:      12,
			Message:         &model.Message{UUID: "M8"},
		}},
		NextSeq: 8,
	}, nil
}

func TestSyncHandlerListSuccess(t *testing.T) {
	stub := &stubSyncService{}
	handler := NewSyncHandler(stub)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/sync?after_seq=7&limit=20", nil)
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U200"})

	handler.List(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if stub.userUUID != "U200" || stub.afterSeq != 7 || stub.limit != 20 {
		t.Fatalf("unexpected sync query: %+v", stub)
	}
	var response struct {
		Data struct {
			NextSeq uint64 `json:"next_seq"`
			Items   []struct {
				SyncSeq     uint64 `json:"sync_seq"`
				MessageUUID string `json:"message_uuid"`
				MessageSeq  uint64 `json:"message_seq"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if response.Data.NextSeq != 8 || len(response.Data.Items) != 1 || response.Data.Items[0].SyncSeq != 8 || response.Data.Items[0].MessageUUID != "M8" || response.Data.Items[0].MessageSeq != 12 {
		t.Fatalf("unexpected response: %s", recorder.Body.String())
	}
}

func TestSyncHandlerRejectsInvalidCursor(t *testing.T) {
	handler := NewSyncHandler(&stubSyncService{})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/sync?after_seq=bad", nil)
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U200"})

	handler.List(context)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
}

func TestSyncHandlerListsGroupCheckpoints(t *testing.T) {
	stub := &stubSyncService{}
	handler := NewSyncHandler(stub)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/sync/groups/checkpoints?group_id=G1&group_id=G2", nil)
	context.Request.Header.Set("X-Device-ID", "web-1")
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U1"})
	handler.ListGroupCheckpoints(context)
	if recorder.Code != http.StatusOK || stub.userUUID != "U1" || stub.deviceID != "web-1" || len(stub.groupUUIDs) != 2 {
		t.Fatalf("unexpected group checkpoint query: status=%d stub=%+v body=%s", recorder.Code, stub, recorder.Body.String())
	}
}

func TestSyncHandlerAdvancesGroupCheckpoint(t *testing.T) {
	stub := &stubSyncService{}
	handler := NewSyncHandler(stub)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/sync/groups/G1/checkpoint", strings.NewReader(`{"message_seq":9}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Request.Header.Set("X-Device-ID", "web-1")
	context.Params = gin.Params{{Key: "group_uuid", Value: "G1"}}
	context.Set(middleware.ContextUserKey, &model.User{UUID: "U1"})
	handler.AdvanceGroupCheckpoint(context)
	if recorder.Code != http.StatusOK || stub.groupUUID != "G1" || stub.checkpointSeq != 9 {
		t.Fatalf("unexpected group checkpoint advance: status=%d stub=%+v body=%s", recorder.Code, stub, recorder.Body.String())
	}
}
