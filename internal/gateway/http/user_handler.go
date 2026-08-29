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
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/service"
)

type userService interface {
	GetByUUID(uuid string) (*model.User, error)
	GetAvatarResponse(targetUUID string) (*service.AvatarResponse, error)
	ListUsersForAdmin(currentUser *model.User, input service.AdminListUsersInput) ([]*model.User, error)
	SearchUsers(currentUser *model.User, input service.SearchUsersInput) ([]*model.User, error)
	UpdateProfile(currentUser *model.User, targetUUID string, input service.UpdateProfileInput) (*model.User, error)
	UploadAvatar(currentUser *model.User, targetUUID string, header *multipart.FileHeader) (*model.User, error)
	UpdateStatus(currentUser *model.User, targetUUID string, status int8) (*model.User, error)
}

type UserHandler struct {
	service        userService
	maxUploadBytes int64
}

func NewUserHandler(service userService) *UserHandler {
	return &UserHandler{service: service, maxUploadBytes: 5 * 1024 * 1024}
}

func (h *UserHandler) WithAvatarMaxUploadBytes(maxUploadBytes int64) *UserHandler {
	if maxUploadBytes > 0 {
		h.maxUploadBytes = maxUploadBytes
	}
	return h
}

// GetCurrent godoc
// @Summary 获取当前用户资料
// @Tags User
// @Security BearerAuth
// @Produce json
// @Success 200 {object} PrivateUserResponseEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Router /users/me [get]
func (h *UserHandler) GetCurrent(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	Success(c, httpdto.PresentUserForViewer(user, user))
}

// Search godoc
// @Summary 搜索用户
// @Tags User
// @Security BearerAuth
// @Produce json
// @Param keyword query string false "搜索关键词"
// @Param limit query int false "返回数量"
// @Success 200 {object} PublicUserListResponseEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 500 {object} ErrorEnvelope
// @Router /users [get]
func (h *UserHandler) Search(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	input := service.SearchUsersInput{
		Keyword: c.Query("keyword"),
		Limit:   queryInt(c, "limit"),
	}

	users, err := h.service.SearchUsers(currentUser, input)
	if err != nil {
		ErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
		return
	}

	Success(c, httpdto.PresentUsersForViewer(currentUser, users))
}

// GetByUUID godoc
// @Summary 获取用户资料
// @Tags User
// @Security BearerAuth
// @Produce json
// @Param uuid path string true "用户 UUID"
// @Success 200 {object} PublicUserResponseEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 404 {object} ErrorEnvelope
// @Failure 500 {object} ErrorEnvelope
// @Router /users/{uuid} [get]
func (h *UserHandler) GetByUUID(c *gin.Context) {
	currentUser, _ := middleware.CurrentUser(c)
	user, err := h.service.GetByUUID(c.Param("uuid"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUserNotFound):
			ErrorWithCode(c, http.StatusNotFound, code.UserNotFound, "user not found")
		default:
			ErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
		}
		return
	}

	Success(c, httpdto.PresentUserForViewer(currentUser, user))
}

// GetAvatar godoc
// @Summary 获取用户头像
// @Tags User
// @Produce image/png
// @Param uuid path string true "用户 UUID"
// @Success 200 {file} binary
// @Failure 404 {object} ErrorEnvelope
// @Failure 503 {object} ErrorEnvelope
// @Failure 500 {object} ErrorEnvelope
// @Router /users/{uuid}/avatar [get]
func (h *UserHandler) GetAvatar(c *gin.Context) {
	avatar, err := h.service.GetAvatarResponse(c.Param("uuid"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUserNotFound):
			ErrorWithCode(c, http.StatusNotFound, code.UserNotFound, "user not found")
		case errors.Is(err, service.ErrAvatarStorageUnavailable):
			ErrorWithCode(c, http.StatusServiceUnavailable, code.FileStorageUnavailable, "avatar storage is unavailable")
		default:
			ErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
		}
		return
	}

	if avatar == nil {
		ErrorWithCode(c, http.StatusInternalServerError, code.Internal, "avatar response is empty")
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

// ListForAdmin godoc
// @Summary 管理员查询用户列表
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Param keyword query string false "搜索关键词"
// @Param status query int false "用户状态"
// @Param limit query int false "返回数量"
// @Success 200 {object} PrivateUserListResponseEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 403 {object} ErrorEnvelope
// @Failure 400 {object} ErrorEnvelope
// @Failure 500 {object} ErrorEnvelope
// @Router /admin/users [get]
func (h *UserHandler) ListForAdmin(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	status, err := queryOptionalStatus(c)
	if err != nil {
		ErrorWithCode(c, http.StatusBadRequest, code.UserInvalidStatus, "status is invalid")
		return
	}

	input := service.AdminListUsersInput{
		Keyword: c.Query("keyword"),
		Status:  status,
		Limit:   queryInt(c, "limit"),
	}

	users, err := h.service.ListUsersForAdmin(currentUser, input)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAdminRequired):
			ErrorWithCode(c, http.StatusForbidden, code.UserAdminRequired, "admin permission is required")
		case errors.Is(err, service.ErrInvalidUserStatus):
			ErrorWithCode(c, http.StatusBadRequest, code.UserInvalidStatus, "status is invalid")
		default:
			ErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
		}
		return
	}

	Success(c, httpdto.PresentUsersForViewer(currentUser, users))
}

// UpdateProfile godoc
// @Summary 更新用户资料
// @Tags User
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param uuid path string true "用户 UUID"
// @Param request body httpdto.UpdateProfileRequest true "资料更新内容"
// @Success 200 {object} PrivateUserResponseEnvelope
// @Failure 400 {object} ErrorEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 403 {object} ErrorEnvelope
// @Failure 404 {object} ErrorEnvelope
// @Failure 500 {object} ErrorEnvelope
// @Router /users/{uuid}/profile [patch]
func (h *UserHandler) UpdateProfile(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	var request httpdto.UpdateProfileRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ErrorWithCode(c, http.StatusBadRequest, code.BadRequest, err.Error())
		return
	}

	user, err := h.service.UpdateProfile(currentUser, c.Param("uuid"), request.ToInput())
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUserPermissionDenied):
			ErrorWithCode(c, http.StatusForbidden, code.UserPermissionDenied, "cannot update another user's profile")
		case errors.Is(err, service.ErrUserNotFound):
			ErrorWithCode(c, http.StatusNotFound, code.UserNotFound, "user not found")
		case errors.Is(err, service.ErrEmptyProfileUpdate):
			ErrorWithCode(c, http.StatusBadRequest, code.UserEmptyProfile, "at least one profile field is required")
		case errors.Is(err, service.ErrInvalidNickname):
			ErrorWithCode(c, http.StatusBadRequest, code.UserInvalidNickname, "nickname must be between 2 and 20 characters")
		case errors.Is(err, service.ErrInvalidEmail):
			ErrorWithCode(c, http.StatusBadRequest, code.UserInvalidEmail, "email format is invalid")
		case errors.Is(err, service.ErrInvalidAvatar):
			ErrorWithCode(c, http.StatusBadRequest, code.UserInvalidAvatar, "avatar is invalid")
		case errors.Is(err, service.ErrInvalidSignature):
			ErrorWithCode(c, http.StatusBadRequest, code.UserInvalidSignature, "signature is invalid")
		default:
			ErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
		}
		return
	}

	Success(c, httpdto.PresentUserForViewer(currentUser, user))
}

// UploadAvatar godoc
// @Summary 上传用户头像
// @Tags User
// @Security BearerAuth
// @Accept multipart/form-data
// @Produce json
// @Param uuid path string true "用户 UUID"
// @Param avatar formData file true "头像文件"
// @Success 200 {object} PrivateUserResponseEnvelope
// @Failure 400 {object} ErrorEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 403 {object} ErrorEnvelope
// @Failure 404 {object} ErrorEnvelope
// @Failure 503 {object} ErrorEnvelope
// @Failure 500 {object} ErrorEnvelope
// @Router /users/{uuid}/avatar [post]
func (h *UserHandler) UploadAvatar(c *gin.Context) {
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
			ErrorWithCode(c, http.StatusBadRequest, code.FileTooLarge, "avatar is too large")
			return
		}
		ErrorWithCode(c, http.StatusBadRequest, code.UserInvalidAvatar, "avatar file is required")
		return
	}

	user, err := h.service.UploadAvatar(currentUser, c.Param("uuid"), fileHeader)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUserPermissionDenied):
			ErrorWithCode(c, http.StatusForbidden, code.UserPermissionDenied, "cannot update another user's avatar")
		case errors.Is(err, service.ErrUserNotFound):
			ErrorWithCode(c, http.StatusNotFound, code.UserNotFound, "user not found")
		case errors.Is(err, service.ErrAvatarMissing), errors.Is(err, service.ErrInvalidAvatar):
			ErrorWithCode(c, http.StatusBadRequest, code.UserInvalidAvatar, "avatar is invalid")
		case errors.Is(err, service.ErrAvatarTooLarge):
			ErrorWithCode(c, http.StatusBadRequest, code.FileTooLarge, "avatar is too large")
		case errors.Is(err, service.ErrAvatarStorageUnavailable):
			ErrorWithCode(c, http.StatusServiceUnavailable, code.FileStorageUnavailable, "avatar storage is unavailable")
		default:
			ErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
		}
		return
	}

	Success(c, httpdto.PresentUserForViewer(currentUser, user))
}

// UpdateStatus godoc
// @Summary 管理员修改用户状态
// @Tags Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param uuid path string true "用户 UUID"
// @Param request body httpdto.UpdateStatusRequest true "状态更新内容"
// @Success 200 {object} PrivateUserResponseEnvelope
// @Failure 400 {object} ErrorEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 403 {object} ErrorEnvelope
// @Failure 404 {object} ErrorEnvelope
// @Failure 500 {object} ErrorEnvelope
// @Router /admin/users/{uuid}/status [patch]
func (h *UserHandler) UpdateStatus(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	var request httpdto.UpdateStatusRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ErrorWithCode(c, http.StatusBadRequest, code.BadRequest, err.Error())
		return
	}

	user, err := h.service.UpdateStatus(currentUser, c.Param("uuid"), request.Status)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAdminRequired):
			ErrorWithCode(c, http.StatusForbidden, code.UserAdminRequired, "admin permission is required")
		case errors.Is(err, service.ErrInvalidUserStatus):
			ErrorWithCode(c, http.StatusBadRequest, code.UserInvalidStatus, "status is invalid")
		case errors.Is(err, service.ErrCannotDisableSelf):
			ErrorWithCode(c, http.StatusBadRequest, code.UserSelfStatusChange, "cannot disable current admin user")
		case errors.Is(err, service.ErrUserNotFound):
			ErrorWithCode(c, http.StatusNotFound, code.UserNotFound, "user not found")
		default:
			ErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
		}
		return
	}

	Success(c, httpdto.PresentUserForViewer(currentUser, user))
}

func queryInt(c *gin.Context, key string) int {
	raw := c.Query(key)
	if raw == "" {
		return 0
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}

	return value
}

func queryOptionalStatus(c *gin.Context) (*int8, error) {
	raw := c.Query("status")
	if raw == "" {
		return nil, nil
	}

	value, err := strconv.ParseInt(raw, 10, 8)
	if err != nil {
		return nil, err
	}

	status := int8(value)
	return &status, nil
}
