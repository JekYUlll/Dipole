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

type contactService interface {
	Apply(currentUserUUID string, input service.ApplyContactInput) (*model.ContactApplication, error)
	ListFriends(currentUserUUID string) ([]*service.ContactListItem, error)
	ListIncomingApplications(currentUserUUID string) ([]*service.ContactApplicationView, error)
	ListOutgoingApplications(currentUserUUID string) ([]*service.ContactApplicationView, error)
	HandleApplication(currentUserUUID string, applicationID uint, action string) (*model.ContactApplication, error)
	DeleteFriend(currentUserUUID, friendUUID string) error
	UpdateRemark(currentUserUUID, friendUUID, remark string) (*model.Contact, error)
	UpdateBlockStatus(currentUserUUID, friendUUID string, blocked bool) (*model.Contact, error)
}

type ContactHandler struct {
	service contactService
}

func NewContactHandler(service contactService) *ContactHandler {
	return &ContactHandler{service: service}
}

func (h *ContactHandler) ListFriends(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	items, err := h.service.ListFriends(currentUser.UUID)
	if err != nil {
		ErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
		return
	}

	Success(c, httpdto.ToContactResponses(items))
}

func (h *ContactHandler) Apply(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	var request httpdto.ApplyContactRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ErrorWithCode(c, http.StatusBadRequest, code.BadRequest, err.Error())
		return
	}

	application, err := h.service.Apply(currentUser.UUID, request.ToInput())
	if err != nil {
		switch {
		case errors.Is(err, service.ErrContactTargetRequired):
			ErrorWithCode(c, http.StatusBadRequest, code.ContactTargetRequired, "target_uuid is required")
		case errors.Is(err, service.ErrContactCannotAddSelf):
			ErrorWithCode(c, http.StatusBadRequest, code.ContactCannotAddSelf, "cannot add self as friend")
		case errors.Is(err, service.ErrContactTargetNotFound):
			ErrorWithCode(c, http.StatusNotFound, code.ContactTargetNotFound, "target user not found")
		case errors.Is(err, service.ErrContactTargetUnavailable):
			ErrorWithCode(c, http.StatusBadRequest, code.ContactTargetUnavailable, "target user is unavailable")
		case errors.Is(err, service.ErrContactAlreadyFriends):
			ErrorWithCode(c, http.StatusConflict, code.ContactAlreadyFriends, "users are already friends")
		case errors.Is(err, service.ErrContactApplicationExists):
			ErrorWithCode(c, http.StatusConflict, code.ContactApplicationExists, "contact application already exists")
		default:
			ErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
		}
		return
	}

	Success(c, gin.H{
		"id":      application.ID,
		"status":  application.Status,
		"message": "contact application sent",
	})
}

func (h *ContactHandler) ListApplications(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	box := c.DefaultQuery("box", "incoming")
	var (
		items []*service.ContactApplicationView
		err   error
	)
	switch box {
	case "incoming":
		items, err = h.service.ListIncomingApplications(currentUser.UUID)
	case "outgoing":
		items, err = h.service.ListOutgoingApplications(currentUser.UUID)
	default:
		ErrorWithCode(c, http.StatusBadRequest, code.BadRequest, "box is invalid")
		return
	}
	if err != nil {
		ErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
		return
	}

	Success(c, httpdto.ToContactApplicationResponses(items))
}

func (h *ContactHandler) HandleApplication(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	applicationID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		ErrorWithCode(c, http.StatusBadRequest, code.BadRequest, "application id is invalid")
		return
	}

	var request httpdto.HandleContactApplicationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ErrorWithCode(c, http.StatusBadRequest, code.BadRequest, err.Error())
		return
	}

	application, err := h.service.HandleApplication(currentUser.UUID, uint(applicationID), request.Action)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrContactApplicationNotFound):
			ErrorWithCode(c, http.StatusNotFound, code.ContactApplicationNotFound, "contact application not found")
		case errors.Is(err, service.ErrContactPermissionDenied):
			ErrorWithCode(c, http.StatusForbidden, code.ContactPermissionDenied, "contact application cannot be handled by current user")
		case errors.Is(err, service.ErrContactApplicationHandled):
			ErrorWithCode(c, http.StatusBadRequest, code.ContactApplicationHandled, "contact application has been handled")
		case errors.Is(err, service.ErrContactActionInvalid):
			ErrorWithCode(c, http.StatusBadRequest, code.ContactActionInvalid, "action is invalid")
		default:
			ErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
		}
		return
	}

	Success(c, gin.H{
		"id":        application.ID,
		"status":    application.Status,
		"handledAt": application.HandledAt,
	})
}

func (h *ContactHandler) DeleteFriend(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	if err := h.service.DeleteFriend(currentUser.UUID, c.Param("friend_uuid")); err != nil {
		switch {
		case errors.Is(err, service.ErrContactTargetRequired):
			ErrorWithCode(c, http.StatusBadRequest, code.ContactTargetRequired, "friend_uuid is required")
		case errors.Is(err, service.ErrContactTargetNotFound):
			ErrorWithCode(c, http.StatusNotFound, code.ContactTargetNotFound, "friend relationship not found")
		default:
			ErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
		}
		return
	}

	Success(c, gin.H{
		"message": "friend deleted",
	})
}

func (h *ContactHandler) UpdateRemark(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	var request httpdto.UpdateContactRemarkRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ErrorWithCode(c, http.StatusBadRequest, code.BadRequest, err.Error())
		return
	}

	contact, err := h.service.UpdateRemark(currentUser.UUID, c.Param("friend_uuid"), request.Remark)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrContactTargetRequired):
			ErrorWithCode(c, http.StatusBadRequest, code.ContactTargetRequired, "friend_uuid is required")
		case errors.Is(err, service.ErrContactTargetNotFound):
			ErrorWithCode(c, http.StatusNotFound, code.ContactTargetNotFound, "friend relationship not found")
		case errors.Is(err, service.ErrContactRemarkTooLong):
			ErrorWithCode(c, http.StatusBadRequest, code.ContactRemarkTooLong, "remark is too long")
		default:
			ErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
		}
		return
	}

	Success(c, gin.H{
		"friend_uuid": contact.FriendUUID,
		"remark":      contact.Remark,
		"status":      contact.Status,
	})
}

func (h *ContactHandler) UpdateBlockStatus(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	var request httpdto.UpdateContactBlockStatusRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ErrorWithCode(c, http.StatusBadRequest, code.BadRequest, err.Error())
		return
	}

	contact, err := h.service.UpdateBlockStatus(currentUser.UUID, c.Param("friend_uuid"), request.Blocked)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrContactTargetRequired):
			ErrorWithCode(c, http.StatusBadRequest, code.ContactTargetRequired, "friend_uuid is required")
		case errors.Is(err, service.ErrContactTargetNotFound):
			ErrorWithCode(c, http.StatusNotFound, code.ContactTargetNotFound, "friend relationship not found")
		default:
			ErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
		}
		return
	}

	Success(c, gin.H{
		"friend_uuid": contact.FriendUUID,
		"blocked":     contact.Status == model.ContactStatusBlocked,
		"status":      contact.Status,
	})
}
