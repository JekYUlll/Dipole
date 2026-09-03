package gateway

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/code"
	"github.com/JekYUlll/Dipole/internal/dto/httpdto"
	"github.com/JekYUlll/Dipole/internal/middleware"
	"github.com/JekYUlll/Dipole/internal/model"
	messagedomain "github.com/JekYUlll/Dipole/internal/services/message/domain"
)

// MessageHandler owns the public message read API used by the standalone Gateway.
type MessageHandler struct {
	service application.MessageQuery
}

func NewMessageHandler(service application.MessageQuery) *MessageHandler {
	return &MessageHandler{service: service}
}

func (h *MessageHandler) ListDirect(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		gatewayErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	_, hasBeforeID := c.GetQuery("before_id")
	beforeID, err := gatewayQueryOptionalUint(c, "before_id")
	if err != nil {
		gatewayErrorWithCode(c, http.StatusBadRequest, code.BadRequest, "before_id is invalid")
		return
	}
	_, hasBeforeSeq := c.GetQuery("before_seq")
	beforeSeq, err := gatewayQueryOptionalUint64(c, "before_seq")
	if err != nil {
		gatewayErrorWithCode(c, http.StatusBadRequest, code.BadRequest, "before_seq is invalid")
		return
	}
	_, hasAfterSeq := c.GetQuery("after_seq")
	afterSeq, err := gatewayQueryOptionalUint64(c, "after_seq")
	if err != nil {
		gatewayErrorWithCode(c, http.StatusBadRequest, code.BadRequest, "after_seq is invalid")
		return
	}
	if gatewayBoolCount(hasBeforeID, hasBeforeSeq, hasAfterSeq) > 1 {
		gatewayErrorWithCode(c, http.StatusBadRequest, code.BadRequest, "before_id, before_seq and after_seq cannot be used together")
		return
	}

	var messages []*model.Message
	if hasAfterSeq {
		messages, err = h.service.ListDirectMessagesAfterSeq(currentUser.UUID, c.Param("target_uuid"), afterSeq, gatewayQueryInt(c, "limit"))
	} else if hasBeforeSeq {
		messages, err = h.service.ListDirectMessagesBeforeSeq(currentUser.UUID, c.Param("target_uuid"), beforeSeq, gatewayQueryInt(c, "limit"))
	} else {
		messages, err = h.service.ListDirectMessages(currentUser.UUID, c.Param("target_uuid"), beforeID, gatewayQueryInt(c, "limit"))
	}
	if err != nil {
		handleMessageReadError(c, err, "target_uuid", "target user not found", messagedomain.ErrMessageFriendRequired, code.MessageFriendRequired, "direct message requires friendship")
		return
	}
	gatewaySuccess(c, httpdto.ToMessageResponses(messages))
}

func (h *MessageHandler) ListGroup(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		gatewayErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	_, hasBeforeID := c.GetQuery("before_id")
	beforeID, err := gatewayQueryOptionalUint(c, "before_id")
	if err != nil {
		gatewayErrorWithCode(c, http.StatusBadRequest, code.BadRequest, "before_id is invalid")
		return
	}
	_, hasAfterID := c.GetQuery("after_id")
	afterID, err := gatewayQueryOptionalUint(c, "after_id")
	if err != nil {
		gatewayErrorWithCode(c, http.StatusBadRequest, code.BadRequest, "after_id is invalid")
		return
	}
	_, hasBeforeSeq := c.GetQuery("before_seq")
	beforeSeq, err := gatewayQueryOptionalUint64(c, "before_seq")
	if err != nil {
		gatewayErrorWithCode(c, http.StatusBadRequest, code.BadRequest, "before_seq is invalid")
		return
	}
	_, hasAfterSeq := c.GetQuery("after_seq")
	afterSeq, err := gatewayQueryOptionalUint64(c, "after_seq")
	if err != nil {
		gatewayErrorWithCode(c, http.StatusBadRequest, code.BadRequest, "after_seq is invalid")
		return
	}
	if gatewayBoolCount(hasBeforeID, hasBeforeSeq, hasAfterID, hasAfterSeq) > 1 {
		gatewayErrorWithCode(c, http.StatusBadRequest, code.BadRequest, "before_id, before_seq, after_id and after_seq cannot be used together")
		return
	}

	limit := gatewayQueryInt(c, "limit")
	var messages []*model.Message
	if hasAfterSeq {
		messages, err = h.service.ListGroupMessagesAfterSeq(currentUser.UUID, c.Param("group_uuid"), afterSeq, limit)
	} else if hasBeforeSeq {
		messages, err = h.service.ListGroupMessagesBeforeSeq(currentUser.UUID, c.Param("group_uuid"), beforeSeq, limit)
	} else if hasAfterID {
		messages, err = h.service.ListGroupMessagesAfter(currentUser.UUID, c.Param("group_uuid"), afterID, limit)
	} else {
		messages, err = h.service.ListGroupMessages(currentUser.UUID, c.Param("group_uuid"), beforeID, limit)
	}
	if err != nil {
		handleMessageReadError(c, err, "group_uuid", "group not found", messagedomain.ErrMessageGroupForbidden, code.MessageGroupForbidden, "group message requires membership")
		return
	}
	gatewaySuccess(c, httpdto.ToMessageResponses(messages))
}

func (h *MessageHandler) ListOffline(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		gatewayErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}
	afterID, err := gatewayQueryOptionalUint(c, "after_id")
	if err != nil {
		gatewayErrorWithCode(c, http.StatusBadRequest, code.BadRequest, "after_id is invalid")
		return
	}
	messages, err := h.service.ListOfflineMessages(currentUser.UUID, afterID, gatewayQueryInt(c, "limit"))
	if err != nil {
		gatewayErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
		return
	}
	gatewaySuccess(c, httpdto.ToMessageResponses(messages))
}

func handleMessageReadError(c *gin.Context, err error, requiredField, notFoundMessage string, forbiddenError error, forbiddenCode int, forbiddenMessage string) {
	switch {
	case errors.Is(err, messagedomain.ErrMessageTargetRequired):
		gatewayErrorWithCode(c, http.StatusBadRequest, code.MessageTargetRequired, requiredField+" is required")
	case errors.Is(err, messagedomain.ErrMessageTargetNotFound):
		gatewayErrorWithCode(c, http.StatusNotFound, code.MessageTargetNotFound, notFoundMessage)
	case errors.Is(err, forbiddenError):
		gatewayErrorWithCode(c, http.StatusForbidden, forbiddenCode, forbiddenMessage)
	default:
		gatewayErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
	}
}

func gatewayQueryOptionalUint(c *gin.Context, key string) (uint, error) {
	raw := c.Query(key)
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(value), nil
}

func gatewayBoolCount(values ...bool) int {
	count := 0
	for _, value := range values {
		if value {
			count++
		}
	}
	return count
}
