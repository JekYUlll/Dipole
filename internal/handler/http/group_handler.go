package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/JekYUlll/Dipole/internal/code"
	"github.com/JekYUlll/Dipole/internal/dto/httpdto"
	"github.com/JekYUlll/Dipole/internal/middleware"
	"github.com/JekYUlll/Dipole/internal/service"
)

type groupService interface {
	CreateGroup(currentUserUUID string, input service.CreateGroupInput) (*service.GroupView, error)
	GetGroup(currentUserUUID, groupUUID string) (*service.GroupView, error)
	ListMembers(currentUserUUID, groupUUID string) ([]*service.GroupMemberView, error)
	AddMembers(currentUserUUID, groupUUID string, memberUUIDs []string) ([]*service.GroupMemberView, error)
	LeaveGroup(currentUserUUID, groupUUID string) error
	UpdateGroup(currentUserUUID, groupUUID string, input service.UpdateGroupInput) (*service.GroupView, error)
	RemoveMembers(currentUserUUID, groupUUID string, memberUUIDs []string) error
	DismissGroup(currentUserUUID, groupUUID string) error
}

type GroupHandler struct {
	service groupService
}

func NewGroupHandler(service groupService) *GroupHandler {
	return &GroupHandler{service: service}
}

func (h *GroupHandler) Create(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	var req httpdto.CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorWithCode(c, http.StatusBadRequest, code.BadRequest, err.Error())
		return
	}

	group, err := h.service.CreateGroup(currentUser.UUID, req.ToInput())
	if err != nil {
		h.handleGroupError(c, err)
		return
	}

	Success(c, httpdto.ToGroupResponse(group))
}

func (h *GroupHandler) Get(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	group, err := h.service.GetGroup(currentUser.UUID, c.Param("uuid"))
	if err != nil {
		h.handleGroupError(c, err)
		return
	}

	Success(c, httpdto.ToGroupResponse(group))
}

func (h *GroupHandler) ListMembers(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	members, err := h.service.ListMembers(currentUser.UUID, c.Param("uuid"))
	if err != nil {
		h.handleGroupError(c, err)
		return
	}

	Success(c, httpdto.ToGroupMemberResponses(members))
}

func (h *GroupHandler) AddMembers(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	var req httpdto.AddGroupMembersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorWithCode(c, http.StatusBadRequest, code.BadRequest, err.Error())
		return
	}

	members, err := h.service.AddMembers(currentUser.UUID, c.Param("uuid"), req.MemberUUIDs)
	if err != nil {
		h.handleGroupError(c, err)
		return
	}

	Success(c, httpdto.ToGroupMemberResponses(members))
}

func (h *GroupHandler) Leave(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	if err := h.service.LeaveGroup(currentUser.UUID, c.Param("uuid")); err != nil {
		h.handleGroupError(c, err)
		return
	}

	Success(c, gin.H{
		"message": "left group successfully",
	})
}

func (h *GroupHandler) Update(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	var req httpdto.UpdateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorWithCode(c, http.StatusBadRequest, code.BadRequest, err.Error())
		return
	}

	group, err := h.service.UpdateGroup(currentUser.UUID, c.Param("uuid"), req.ToInput())
	if err != nil {
		h.handleGroupError(c, err)
		return
	}

	Success(c, httpdto.ToGroupResponse(group))
}

func (h *GroupHandler) RemoveMembers(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	var req httpdto.RemoveGroupMembersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorWithCode(c, http.StatusBadRequest, code.BadRequest, err.Error())
		return
	}

	if err := h.service.RemoveMembers(currentUser.UUID, c.Param("uuid"), req.MemberUUIDs); err != nil {
		h.handleGroupError(c, err)
		return
	}

	Success(c, gin.H{
		"message": "group members removed successfully",
	})
}

func (h *GroupHandler) Dismiss(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	if err := h.service.DismissGroup(currentUser.UUID, c.Param("uuid")); err != nil {
		h.handleGroupError(c, err)
		return
	}

	Success(c, gin.H{
		"message": "group dismissed successfully",
	})
}

func (h *GroupHandler) handleGroupError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrGroupEmptyUpdate):
		ErrorWithCode(c, http.StatusBadRequest, code.GroupEmptyUpdate, "group update is empty")
	case errors.Is(err, service.ErrGroupNameRequired):
		ErrorWithCode(c, http.StatusBadRequest, code.GroupNameRequired, "group name is required")
	case errors.Is(err, service.ErrGroupNameTooLong):
		ErrorWithCode(c, http.StatusBadRequest, code.GroupNameTooLong, "group name is too long")
	case errors.Is(err, service.ErrGroupNoticeTooLong):
		ErrorWithCode(c, http.StatusBadRequest, code.GroupNoticeTooLong, "group notice is too long")
	case errors.Is(err, service.ErrGroupAvatarTooLong):
		ErrorWithCode(c, http.StatusBadRequest, code.GroupAvatarTooLong, "group avatar is too long")
	case errors.Is(err, service.ErrGroupNotFound):
		ErrorWithCode(c, http.StatusNotFound, code.GroupNotFound, "group not found")
	case errors.Is(err, service.ErrGroupPermissionDenied):
		ErrorWithCode(c, http.StatusForbidden, code.GroupPermissionDenied, "group permission denied")
	case errors.Is(err, service.ErrGroupMemberRequired):
		ErrorWithCode(c, http.StatusBadRequest, code.GroupMemberRequired, "member_uuids is required")
	case errors.Is(err, service.ErrGroupMemberUnavailable):
		ErrorWithCode(c, http.StatusBadRequest, code.GroupMemberUnavailable, "group member is unavailable")
	case errors.Is(err, service.ErrGroupMemberAlreadyIn):
		ErrorWithCode(c, http.StatusConflict, code.GroupMemberAlreadyIn, "group member already exists")
	case errors.Is(err, service.ErrGroupOwnerCannotLeave):
		ErrorWithCode(c, http.StatusConflict, code.GroupOwnerCannotLeave, "group owner cannot leave")
	case errors.Is(err, service.ErrGroupOwnerCannotBeRemoved):
		ErrorWithCode(c, http.StatusConflict, code.GroupOwnerCannotBeRemoved, "group owner cannot be removed")
	default:
		ErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
	}
}
