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
	MarkDirectConversationRead(userUUID, targetUUID string) error
}

type ConversationHandler struct {
	service conversationService
}

func NewConversationHandler(service conversationService) *ConversationHandler {
	return &ConversationHandler{service: service}
}

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

func (h *ConversationHandler) MarkDirectRead(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	if err := h.service.MarkDirectConversationRead(currentUser.UUID, c.Param("target_uuid")); err != nil {
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

var _ = model.User{}
