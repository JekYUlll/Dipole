package gateway

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/code"
	"github.com/JekYUlll/Dipole/internal/dto/httpdto"
	"github.com/JekYUlll/Dipole/internal/middleware"
	syncdomain "github.com/JekYUlll/Dipole/internal/services/sync/domain"
)

// SyncHandler owns the public Sync API used by the standalone Gateway.
type SyncHandler struct {
	service            application.SyncApplication
	comparisonObserver application.ClientSyncComparisonObserver
}

func NewSyncHandler(service application.SyncApplication) *SyncHandler {
	return &SyncHandler{service: service}
}

func (h *SyncHandler) WithComparisonObserver(observer application.ClientSyncComparisonObserver) *SyncHandler {
	h.comparisonObserver = observer
	return h
}

func (h *SyncHandler) GetCheckpoint(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		gatewayErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}
	checkpoint, err := h.service.GetCheckpoint(currentUser.UUID, c.GetHeader("X-Device-ID"))
	if err != nil {
		handleSyncCheckpointError(c, err)
		return
	}
	gatewaySuccess(c, httpdto.ToDeviceSyncCheckpointResponse(checkpoint))
}

func (h *SyncHandler) AdvanceCheckpoint(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		gatewayErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}
	var request httpdto.AdvanceSyncCheckpointRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		gatewayErrorWithCode(c, http.StatusBadRequest, code.BadRequest, err.Error())
		return
	}
	checkpoint, err := h.service.AdvanceCheckpoint(currentUser.UUID, c.GetHeader("X-Device-ID"), request.SyncSeq)
	if err != nil {
		handleSyncCheckpointError(c, err)
		return
	}
	gatewaySuccess(c, httpdto.ToDeviceSyncCheckpointResponse(checkpoint))
}

func (h *SyncHandler) ListGroupCheckpoints(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		gatewayErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}
	checkpoints, err := h.service.ListGroupCheckpoints(currentUser.UUID, c.GetHeader("X-Device-ID"), c.QueryArray("group_id"))
	if err != nil {
		handleSyncCheckpointError(c, err)
		return
	}
	gatewaySuccess(c, httpdto.ToGroupSyncCheckpointResponses(checkpoints))
}

func (h *SyncHandler) AdvanceGroupCheckpoint(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		gatewayErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}
	var request httpdto.AdvanceGroupSyncCheckpointRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		gatewayErrorWithCode(c, http.StatusBadRequest, code.BadRequest, err.Error())
		return
	}
	checkpoint, err := h.service.AdvanceGroupCheckpoint(currentUser.UUID, c.GetHeader("X-Device-ID"), c.Param("group_uuid"), request.MessageSeq)
	if err != nil {
		handleSyncCheckpointError(c, err)
		return
	}
	gatewaySuccess(c, httpdto.ToGroupSyncCheckpointResponse(checkpoint))
}

func (h *SyncHandler) ReportComparison(c *gin.Context) {
	if _, ok := middleware.CurrentUser(c); !ok {
		gatewayErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}
	deviceID := strings.TrimSpace(c.GetHeader("X-Device-ID"))
	if deviceID == "" || len(deviceID) > 128 {
		gatewayErrorWithCode(c, http.StatusBadRequest, code.BadRequest, "device ID is invalid")
		return
	}
	var request httpdto.ClientSyncComparisonRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		gatewayErrorWithCode(c, http.StatusBadRequest, code.BadRequest, "comparison report is invalid")
		return
	}
	counts := map[string]int{
		"match": request.Match, "pending": request.Pending, "legacy_only": request.LegacyOnly, "sync_only": request.SyncOnly,
		"overflow": request.Overflow, "storage_full": request.StorageFull, "sync_error": request.SyncError,
		"timeline_match": request.TimelineMatch, "timeline_missing": request.TimelineMissing, "timeline_mismatch": request.TimelineMismatch,
		"timeline_error": request.TimelineError, "timeline_invalid": request.TimelineInvalid,
	}
	for _, count := range counts {
		if count < 0 || count > 10000 {
			gatewayErrorWithCode(c, http.StatusBadRequest, code.BadRequest, "comparison count is invalid")
			return
		}
	}
	if h.comparisonObserver != nil {
		if request.Baseline {
			h.comparisonObserver.ObserveClientSyncComparison("baseline", 1)
		}
		for outcome, count := range counts {
			h.comparisonObserver.ObserveClientSyncComparison(outcome, count)
		}
	}
	gatewaySuccess(c, gin.H{})
}

func (h *SyncHandler) List(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		gatewayErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}
	afterSeq, err := gatewayQueryOptionalUint64(c, "after_seq")
	if err != nil {
		gatewayErrorWithCode(c, http.StatusBadRequest, code.BadRequest, "after_seq is invalid")
		return
	}
	page, err := h.service.List(currentUser.UUID, afterSeq, gatewayQueryInt(c, "limit"))
	if err != nil {
		gatewayErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
		return
	}
	gatewaySuccess(c, httpdto.ToSyncPageResponse(page))
}

func handleSyncCheckpointError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, syncdomain.ErrSyncGroupForbidden):
		gatewayErrorWithCode(c, http.StatusForbidden, code.Forbidden, err.Error())
	case errors.Is(err, syncdomain.ErrSyncDeviceIDRequired), errors.Is(err, syncdomain.ErrSyncDeviceIDInvalid), errors.Is(err, syncdomain.ErrSyncCheckpointAhead):
		gatewayErrorWithCode(c, http.StatusBadRequest, code.BadRequest, err.Error())
	default:
		gatewayErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
	}
}

func gatewayQueryOptionalUint64(c *gin.Context, key string) (uint64, error) {
	raw := c.Query(key)
	if raw == "" {
		return 0, nil
	}
	return strconv.ParseUint(raw, 10, 64)
}

func gatewayQueryInt(c *gin.Context, key string) int {
	value, err := strconv.Atoi(c.DefaultQuery(key, "0"))
	if err != nil {
		return 0
	}
	return value
}

func gatewaySuccess(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"code": code.Success, "data": data})
}

func gatewayErrorWithCode(c *gin.Context, statusCode, businessCode int, message string) {
	c.JSON(statusCode, gin.H{"code": businessCode, "message": message})
}
