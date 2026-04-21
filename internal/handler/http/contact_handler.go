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

// ListFriends godoc
// @Summary 获取好友列表
// @Tags Contact
// @Security BearerAuth
// @Produce json
// @Success 200 {object} ContactListResponseEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 500 {object} ErrorEnvelope
// @Router /contacts [get]
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

// Apply godoc
// @Summary 发起好友申请
// @Tags Contact
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body httpdto.ApplyContactRequest true "好友申请"
// @Success 200 {object} IDStatusResponseEnvelope
// @Failure 400 {object} ErrorEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 404 {object} ErrorEnvelope
// @Failure 409 {object} ErrorEnvelope
// @Failure 500 {object} ErrorEnvelope
// @Router /contacts/applications [post]
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

// ListApplications godoc
// @Summary 获取好友申请列表
// @Tags Contact
// @Security BearerAuth
// @Produce json
// @Param box query string false "incoming 或 outgoing"
// @Success 200 {object} ContactApplicationListResponseEnvelope
// @Failure 400 {object} ErrorEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 500 {object} ErrorEnvelope
// @Router /contacts/applications [get]
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

// HandleApplication godoc
// @Summary 处理好友申请
// @Tags Contact
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "申请 ID"
// @Param request body httpdto.HandleContactApplicationRequest true "处理动作"
// @Success 200 {object} IDStatusResponseEnvelope
// @Failure 400 {object} ErrorEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 403 {object} ErrorEnvelope
// @Failure 404 {object} ErrorEnvelope
// @Failure 500 {object} ErrorEnvelope
// @Router /contacts/applications/{id} [patch]
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
		case errors.Is(err, service.ErrContactApplicationExpired):
			ErrorWithCode(c, http.StatusBadRequest, code.ContactApplicationExpired, "contact application has expired")
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

// DeleteFriend godoc
// @Summary 删除好友
// @Tags Contact
// @Security BearerAuth
// @Produce json
// @Param friend_uuid path string true "好友 UUID"
// @Success 200 {object} MessageOnlyResponseEnvelope
// @Failure 400 {object} ErrorEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 404 {object} ErrorEnvelope
// @Failure 500 {object} ErrorEnvelope
// @Router /contacts/{friend_uuid} [delete]
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

// UpdateRemark godoc
// @Summary 更新好友备注
// @Tags Contact
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param friend_uuid path string true "好友 UUID"
// @Param request body httpdto.UpdateContactRemarkRequest true "备注内容"
// @Success 200 {object} ContactRemarkResponseEnvelope
// @Failure 400 {object} ErrorEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 404 {object} ErrorEnvelope
// @Failure 500 {object} ErrorEnvelope
// @Router /contacts/{friend_uuid}/remark [patch]
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

// UpdateBlockStatus godoc
// @Summary 更新好友拉黑状态
// @Tags Contact
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param friend_uuid path string true "好友 UUID"
// @Param request body httpdto.UpdateContactBlockStatusRequest true "拉黑状态"
// @Success 200 {object} ContactBlockResponseEnvelope
// @Failure 400 {object} ErrorEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 404 {object} ErrorEnvelope
// @Failure 500 {object} ErrorEnvelope
// @Router /contacts/{friend_uuid}/block [patch]
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
