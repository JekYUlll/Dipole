package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/JekYUlll/Dipole/internal/code"
	"github.com/JekYUlll/Dipole/internal/dto/httpdto"
	"github.com/JekYUlll/Dipole/internal/middleware"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/service"
)

type conversationService interface {
	ListForUser(userUUID string, limit int) ([]*service.ConversationView, error)
	MarkDirectConversationRead(userUUID, targetUUID string) (*service.ConversationReadReceipt, error)
	MarkGroupConversationRead(userUUID, groupUUID string) error
	UpdateGroupRemark(userUUID, groupUUID, remark string) (*model.Conversation, error)
}

type ConversationHandler struct {
	service conversationService
}

func NewConversationHandler(service conversationService) *ConversationHandler {
	return &ConversationHandler{service: service}
}

// List godoc
// @Summary 获取会话列表
// @Tags Conversation
// @Security BearerAuth
// @Produce json
// @Param limit query int false "返回数量"
// @Success 200 {object} ConversationListResponseEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 500 {object} ErrorEnvelope
// @Router /conversations [get]
func (h *ConversationHandler) List(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	conversations, err := h.service.ListForUser(currentUser.UUID, queryInt(c, "limit"))
	if err != nil {
		ErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
		return
	}

	Success(c, httpdto.ToConversationResponses(conversations))
}

// MarkDirectRead godoc
// @Summary 标记单聊已读
// @Tags Conversation
// @Security BearerAuth
// @Produce json
// @Param target_uuid path string true "目标用户 UUID"
// @Success 200 {object} MessageOnlyResponseEnvelope
// @Failure 400 {object} ErrorEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 404 {object} ErrorEnvelope
// @Failure 500 {object} ErrorEnvelope
// @Router /conversations/direct/{target_uuid}/read [patch]
func (h *ConversationHandler) MarkDirectRead(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	if _, err := h.service.MarkDirectConversationRead(currentUser.UUID, c.Param("target_uuid")); err != nil {
		switch {
		case errors.Is(err, service.ErrConversationTargetRequired):
			ErrorWithCode(c, http.StatusBadRequest, code.ConversationTargetRequired, "target_uuid is required")
		case errors.Is(err, service.ErrConversationTargetNotFound):
			ErrorWithCode(c, http.StatusNotFound, code.ConversationTargetNotFound, "target user not found")
		default:
			ErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
		}
		return
	}

	Success(c, gin.H{
		"message": "conversation marked as read",
	})
}

// MarkGroupRead godoc
// @Summary 标记群会话已读
// @Tags Conversation
// @Security BearerAuth
// @Produce json
// @Param group_uuid path string true "群 UUID"
// @Success 200 {object} MessageOnlyResponseEnvelope
// @Failure 400 {object} ErrorEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 403 {object} ErrorEnvelope
// @Failure 404 {object} ErrorEnvelope
// @Failure 500 {object} ErrorEnvelope
// @Router /conversations/group/{group_uuid}/read [patch]
func (h *ConversationHandler) MarkGroupRead(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	if err := h.service.MarkGroupConversationRead(currentUser.UUID, c.Param("group_uuid")); err != nil {
		switch {
		case errors.Is(err, service.ErrConversationTargetRequired):
			ErrorWithCode(c, http.StatusBadRequest, code.ConversationTargetRequired, "group_uuid is required")
		case errors.Is(err, service.ErrConversationTargetNotFound):
			ErrorWithCode(c, http.StatusNotFound, code.GroupNotFound, "group not found")
		case errors.Is(err, service.ErrConversationPermissionDenied):
			ErrorWithCode(c, http.StatusForbidden, code.GroupPermissionDenied, "group permission denied")
		default:
			ErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
		}
		return
	}

	Success(c, gin.H{
		"message": "conversation marked as read",
	})
}

// UpdateGroupRemark godoc
// @Summary 更新群会话备注
// @Tags Conversation
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param group_uuid path string true "群 UUID"
// @Param request body httpdto.UpdateConversationRemarkRequest true "备注内容"
// @Success 200 {object} ConversationRemarkResponseEnvelope
// @Failure 400 {object} ErrorEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 403 {object} ErrorEnvelope
// @Failure 404 {object} ErrorEnvelope
// @Failure 500 {object} ErrorEnvelope
// @Router /conversations/group/{group_uuid}/remark [patch]
func (h *ConversationHandler) UpdateGroupRemark(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	var req httpdto.UpdateConversationRemarkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorWithCode(c, http.StatusBadRequest, code.BadRequest, err.Error())
		return
	}

	conversation, err := h.service.UpdateGroupRemark(currentUser.UUID, c.Param("group_uuid"), req.Remark)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrConversationTargetRequired):
			ErrorWithCode(c, http.StatusBadRequest, code.ConversationTargetRequired, "group_uuid is required")
		case errors.Is(err, service.ErrConversationTargetNotFound):
			ErrorWithCode(c, http.StatusNotFound, code.GroupNotFound, "group not found")
		case errors.Is(err, service.ErrConversationPermissionDenied):
			ErrorWithCode(c, http.StatusForbidden, code.GroupPermissionDenied, "group permission denied")
		case errors.Is(err, service.ErrConversationRemarkTooLong):
			ErrorWithCode(c, http.StatusBadRequest, code.ConversationRemarkTooLong, "remark is too long")
		default:
			ErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
		}
		return
	}

	Success(c, gin.H{
		"conversation_key": model.GroupConversationKey(c.Param("group_uuid")),
		"remark":           conversation.Remark,
	})
}

var _ = model.User{}
