package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	applicationPort "github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/code"
	"github.com/JekYUlll/Dipole/internal/dto/httpdto"
	"github.com/JekYUlll/Dipole/internal/middleware"
)

type SyncHandler struct {
	service applicationPort.SyncApplication
}

func NewSyncHandler(service applicationPort.SyncApplication) *SyncHandler {
	return &SyncHandler{service: service}
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
