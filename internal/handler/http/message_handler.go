package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/JekYUlll/Dipole/internal/code"
	"github.com/JekYUlll/Dipole/internal/dto/httpdto"
	"github.com/JekYUlll/Dipole/internal/middleware"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/service"
)

type messageService interface {
	ListDirectMessages(currentUserUUID, targetUUID string, beforeID uint, limit int) ([]*model.Message, error)
	ListGroupMessages(currentUserUUID, groupUUID string, beforeID uint, limit int) ([]*model.Message, error)
	ListOfflineMessages(currentUserUUID string, afterID uint, limit int) ([]*model.Message, error)
}

type MessageHandler struct {
	service messageService
}

func NewMessageHandler(service messageService) *MessageHandler {
	return &MessageHandler{service: service}
}

func (h *MessageHandler) ListDirect(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	beforeID, err := queryOptionalUint(c, "before_id")
	if err != nil {
		ErrorWithCode(c, http.StatusBadRequest, code.BadRequest, "before_id is invalid")
		return
	}

	messages, err := h.service.ListDirectMessages(
		currentUser.UUID,
		c.Param("target_uuid"),
		beforeID,
		queryInt(c, "limit"),
	)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrMessageTargetRequired):
			ErrorWithCode(c, http.StatusBadRequest, code.MessageTargetRequired, "target_uuid is required")
		case errors.Is(err, service.ErrMessageTargetNotFound):
			ErrorWithCode(c, http.StatusNotFound, code.MessageTargetNotFound, "target user not found")
		case errors.Is(err, service.ErrMessageFriendRequired):
			ErrorWithCode(c, http.StatusForbidden, code.MessageFriendRequired, "direct message requires friendship")
		default:
			ErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
		}
		return
	}

	Success(c, httpdto.ToMessageResponses(messages))
}

func (h *MessageHandler) ListGroup(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	beforeID, err := queryOptionalUint(c, "before_id")
	if err != nil {
		ErrorWithCode(c, http.StatusBadRequest, code.BadRequest, "before_id is invalid")
		return
	}

	messages, err := h.service.ListGroupMessages(
		currentUser.UUID,
		c.Param("group_uuid"),
		beforeID,
		queryInt(c, "limit"),
	)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrMessageTargetRequired):
			ErrorWithCode(c, http.StatusBadRequest, code.MessageTargetRequired, "group_uuid is required")
		case errors.Is(err, service.ErrMessageTargetNotFound):
			ErrorWithCode(c, http.StatusNotFound, code.MessageTargetNotFound, "group not found")
		case errors.Is(err, service.ErrMessageGroupForbidden):
			ErrorWithCode(c, http.StatusForbidden, code.MessageGroupForbidden, "group message requires membership")
		default:
			ErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
		}
		return
	}

	Success(c, httpdto.ToMessageResponses(messages))
}

func (h *MessageHandler) ListOffline(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	afterID, err := queryOptionalUint(c, "after_id")
	if err != nil {
		ErrorWithCode(c, http.StatusBadRequest, code.BadRequest, "after_id is invalid")
		return
	}

	messages, err := h.service.ListOfflineMessages(currentUser.UUID, afterID, queryInt(c, "limit"))
	if err != nil {
		ErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
		return
	}

	Success(c, httpdto.ToMessageResponses(messages))
}

func queryOptionalUint(c *gin.Context, key string) (uint, error) {
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
