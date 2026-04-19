package http

import (
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/JekYUlll/Dipole/internal/code"
	"github.com/JekYUlll/Dipole/internal/dto/httpdto"
	"github.com/JekYUlll/Dipole/internal/middleware"
	"github.com/JekYUlll/Dipole/internal/service"
)

type groupService interface {
	CreateGroup(currentUserUUID string, input service.CreateGroupInput) (*service.GroupView, error)
	GetGroup(currentUserUUID, groupUUID string) (*service.GroupView, error)
	GetAvatarResponse(groupUUID string) (*service.GroupAvatarResponse, error)
	ListMembers(currentUserUUID, groupUUID string) ([]*service.GroupMemberView, error)
	AddMembers(currentUserUUID, groupUUID string, memberUUIDs []string) ([]*service.GroupMemberView, error)
	LeaveGroup(currentUserUUID, groupUUID string) error
	UpdateGroup(currentUserUUID, groupUUID string, input service.UpdateGroupInput) (*service.GroupView, error)
	UploadAvatar(currentUserUUID, groupUUID string, header *multipart.FileHeader) (*service.GroupView, error)
	RemoveMembers(currentUserUUID, groupUUID string, memberUUIDs []string) error
	DismissGroup(currentUserUUID, groupUUID string) error
}

type GroupHandler struct {
	service        groupService
	maxUploadBytes int64
}

func NewGroupHandler(service groupService) *GroupHandler {
	return &GroupHandler{service: service, maxUploadBytes: 5 * 1024 * 1024}
}

func (h *GroupHandler) WithAvatarMaxUploadBytes(maxUploadBytes int64) *GroupHandler {
	if maxUploadBytes > 0 {
		h.maxUploadBytes = maxUploadBytes
	}
	return h
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

func (h *GroupHandler) GetAvatar(c *gin.Context) {
	avatar, err := h.service.GetAvatarResponse(c.Param("uuid"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrGroupNotFound):
			ErrorWithCode(c, http.StatusNotFound, code.GroupNotFound, "group not found")
		case errors.Is(err, service.ErrGroupAvatarMissing):
			ErrorWithCode(c, http.StatusNotFound, code.GroupNotFound, "group avatar not found")
		case errors.Is(err, service.ErrGroupAvatarStorageUnavailable):
			ErrorWithCode(c, http.StatusServiceUnavailable, code.FileStorageUnavailable, "group avatar storage is unavailable")
		default:
			ErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
		}
		return
	}
	if avatar == nil {
		ErrorWithCode(c, http.StatusInternalServerError, code.Internal, "group avatar response is empty")
		return
	}
	if avatar.Cleanup != nil {
		defer avatar.Cleanup()
	}
	if avatar.Content != nil {
		defer avatar.Content.Close()
		if avatar.ContentType != "" {
			c.Header("Content-Type", avatar.ContentType)
		}
		if avatar.ContentSize > 0 {
			c.Header("Content-Length", strconv.FormatInt(avatar.ContentSize, 10))
		}
		c.Header("Cache-Control", "private, max-age=60")
		c.Status(http.StatusOK)
		if _, err := io.Copy(c.Writer, avatar.Content); err != nil {
			_ = c.Error(err)
		}
		return
	}

	c.Redirect(http.StatusFound, avatar.RedirectURL)
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

func (h *GroupHandler) UploadAvatar(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}
	if h.maxUploadBytes > 0 {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.maxUploadBytes)
	}

	fileHeader, err := c.FormFile("avatar")
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "request body too large") {
			ErrorWithCode(c, http.StatusBadRequest, code.GroupAvatarTooLarge, "group avatar is too large")
			return
		}
		ErrorWithCode(c, http.StatusBadRequest, code.GroupAvatarInvalid, "group avatar file is required")
		return
	}

	group, err := h.service.UploadAvatar(currentUser.UUID, c.Param("uuid"), fileHeader)
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
	case errors.Is(err, service.ErrGroupDismissed):
		ErrorWithCode(c, http.StatusConflict, code.GroupDismissed, "group has been dismissed")
	case errors.Is(err, service.ErrGroupAvatarMissing), errors.Is(err, service.ErrGroupAvatarInvalid):
		ErrorWithCode(c, http.StatusBadRequest, code.GroupAvatarInvalid, "group avatar is invalid")
	case errors.Is(err, service.ErrGroupAvatarTooLarge):
		ErrorWithCode(c, http.StatusBadRequest, code.GroupAvatarTooLarge, "group avatar is too large")
	case errors.Is(err, service.ErrGroupAvatarStorageUnavailable):
		ErrorWithCode(c, http.StatusServiceUnavailable, code.GroupAvatarStorageUnavailable, "group avatar storage is unavailable")
	default:
		ErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
	}
}
