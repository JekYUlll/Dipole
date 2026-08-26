package http

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	applicationPort "github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/code"
	"github.com/JekYUlll/Dipole/internal/dto/httpdto"
	"github.com/JekYUlll/Dipole/internal/middleware"
	"github.com/JekYUlll/Dipole/internal/service"
)

type SyncHandler struct {
	service            applicationPort.SyncApplication
	comparisonObserver applicationPort.ClientSyncComparisonObserver
}

// GetCheckpoint godoc
// @Summary 获取当前设备同步游标
// @Tags Sync
// @Security BearerAuth
// @Produce json
// @Param X-Device-ID header string true "稳定设备 ID"
// @Success 200 {object} DeviceSyncCheckpointResponseEnvelope
// @Failure 400 {object} ErrorEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Router /sync/checkpoint [get]
func (h *SyncHandler) GetCheckpoint(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}
	checkpoint, err := h.service.GetCheckpoint(currentUser.UUID, c.GetHeader("X-Device-ID"))
	if err != nil {
		handleSyncCheckpointError(c, err)
		return
	}
	Success(c, httpdto.ToDeviceSyncCheckpointResponse(checkpoint))
}

// AdvanceCheckpoint godoc
// @Summary 确认当前设备同步游标
// @Tags Sync
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param X-Device-ID header string true "稳定设备 ID"
// @Param request body httpdto.AdvanceSyncCheckpointRequest true "已持久化的同步游标"
// @Success 200 {object} DeviceSyncCheckpointResponseEnvelope
// @Failure 400 {object} ErrorEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Router /sync/checkpoint [patch]
func (h *SyncHandler) AdvanceCheckpoint(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}
	var request httpdto.AdvanceSyncCheckpointRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ErrorWithCode(c, http.StatusBadRequest, code.BadRequest, err.Error())
		return
	}
	checkpoint, err := h.service.AdvanceCheckpoint(currentUser.UUID, c.GetHeader("X-Device-ID"), request.SyncSeq)
	if err != nil {
		handleSyncCheckpointError(c, err)
		return
	}
	Success(c, httpdto.ToDeviceSyncCheckpointResponse(checkpoint))
}

func handleSyncCheckpointError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrSyncGroupForbidden):
		ErrorWithCode(c, http.StatusForbidden, code.Forbidden, err.Error())
	case errors.Is(err, service.ErrSyncDeviceIDRequired), errors.Is(err, service.ErrSyncDeviceIDInvalid), errors.Is(err, service.ErrSyncCheckpointAhead):
		ErrorWithCode(c, http.StatusBadRequest, code.BadRequest, err.Error())
	default:
		ErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
	}
}

// ListGroupCheckpoints godoc
// @Summary 查询设备的群消息同步位点
// @Tags Sync
// @Security BearerAuth
// @Produce json
// @Param X-Device-ID header string true "稳定设备 ID"
// @Param group_id query []string true "客户端已知群 ID"
// @Success 200 {object} GroupSyncCheckpointListResponseEnvelope
// @Failure 400 {object} ErrorEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 403 {object} ErrorEnvelope
// @Router /sync/groups/checkpoints [get]
func (h *SyncHandler) ListGroupCheckpoints(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}
	checkpoints, err := h.service.ListGroupCheckpoints(currentUser.UUID, c.GetHeader("X-Device-ID"), c.QueryArray("group_id"))
	if err != nil {
		handleSyncCheckpointError(c, err)
		return
	}
	Success(c, httpdto.ToGroupSyncCheckpointResponses(checkpoints))
}

// AdvanceGroupCheckpoint godoc
// @Summary 确认设备已持久化的群消息位点
// @Tags Sync
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param X-Device-ID header string true "稳定设备 ID"
// @Param group_uuid path string true "群 ID"
// @Param request body httpdto.AdvanceGroupSyncCheckpointRequest true "已持久化的群消息序号"
// @Success 200 {object} GroupSyncCheckpointResponseEnvelope
// @Failure 400 {object} ErrorEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 403 {object} ErrorEnvelope
// @Router /sync/groups/{group_uuid}/checkpoint [patch]
func (h *SyncHandler) AdvanceGroupCheckpoint(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}
	var request httpdto.AdvanceGroupSyncCheckpointRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ErrorWithCode(c, http.StatusBadRequest, code.BadRequest, err.Error())
		return
	}
	checkpoint, err := h.service.AdvanceGroupCheckpoint(currentUser.UUID, c.GetHeader("X-Device-ID"), c.Param("group_uuid"), request.MessageSeq)
	if err != nil {
		handleSyncCheckpointError(c, err)
		return
	}
	Success(c, httpdto.ToGroupSyncCheckpointResponse(checkpoint))
}

func NewSyncHandler(service applicationPort.SyncApplication) *SyncHandler {
	return &SyncHandler{service: service}
}

func (h *SyncHandler) WithComparisonObserver(observer applicationPort.ClientSyncComparisonObserver) *SyncHandler {
	h.comparisonObserver = observer
	return h
}

// ReportComparison godoc
// @Summary 上报 Web 同步协议聚合观测结果
// @Description 仅接收对照与客户端错误计数，不接收消息 ID 或正文
// @Tags Sync
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param X-Device-ID header string true "稳定设备 ID"
// @Param request body httpdto.ClientSyncComparisonRequest true "旧 Offline 与 Sync 聚合对照结果"
// @Success 200 {object} SuccessEnvelope
// @Failure 400 {object} ErrorEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Router /sync/comparison [post]
func (h *SyncHandler) ReportComparison(c *gin.Context) {
	if _, ok := middleware.CurrentUser(c); !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}
	deviceID := strings.TrimSpace(c.GetHeader("X-Device-ID"))
	if deviceID == "" || len(deviceID) > 128 {
		ErrorWithCode(c, http.StatusBadRequest, code.BadRequest, "device ID is invalid")
		return
	}
	var request httpdto.ClientSyncComparisonRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ErrorWithCode(c, http.StatusBadRequest, code.BadRequest, "comparison report is invalid")
		return
	}
	counts := map[string]int{
		"match":        request.Match,
		"pending":      request.Pending,
		"legacy_only":  request.LegacyOnly,
		"sync_only":    request.SyncOnly,
		"overflow":     request.Overflow,
		"storage_full": request.StorageFull,
		"sync_error":   request.SyncError,
	}
	for _, count := range counts {
		if count < 0 || count > 10000 {
			ErrorWithCode(c, http.StatusBadRequest, code.BadRequest, "comparison count is invalid")
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
	Success(c, gin.H{})
}

// List godoc
// @Summary 增量同步用户消息
// @Tags Sync
// @Security BearerAuth
// @Produce json
// @Param after_seq query int false "同步游标"
// @Param limit query int false "返回数量"
// @Success 200 {object} SyncPageResponseEnvelope
// @Failure 400 {object} ErrorEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 500 {object} ErrorEnvelope
// @Router /sync [get]
func (h *SyncHandler) List(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	afterSeq, err := queryOptionalUint64(c, "after_seq")
	if err != nil {
		ErrorWithCode(c, http.StatusBadRequest, code.BadRequest, "after_seq is invalid")
		return
	}
	page, err := h.service.List(currentUser.UUID, afterSeq, queryInt(c, "limit"))
	if err != nil {
		ErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
		return
	}
	Success(c, httpdto.ToSyncPageResponse(page))
}

func queryOptionalUint64(c *gin.Context, key string) (uint64, error) {
	raw := c.Query(key)
	if raw == "" {
		return 0, nil
	}
	return strconv.ParseUint(raw, 10, 64)
}
