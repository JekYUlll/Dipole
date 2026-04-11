package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/JekYUlll/Dipole/internal/code"
	"github.com/JekYUlll/Dipole/internal/middleware"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/service"
)

type userService interface {
	GetByUUID(uuid string) (*model.User, error)
	ListUsersForAdmin(currentUser *model.User, input service.AdminListUsersInput) ([]*model.User, error)
	SearchUsers(currentUser *model.User, input service.SearchUsersInput) ([]*model.User, error)
	UpdateProfile(currentUser *model.User, targetUUID string, input service.UpdateProfileInput) (*model.User, error)
	UpdateStatus(currentUser *model.User, targetUUID string, status int8) (*model.User, error)
}

type UserHandler struct {
	service userService
}

func NewUserHandler(service userService) *UserHandler {
	return &UserHandler{service: service}
}

func (h *UserHandler) GetCurrent(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	Success(c, presentUserForViewer(user, user))
}

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

	Success(c, presentUsersForViewer(currentUser, users))
}

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

	Success(c, presentUserForViewer(currentUser, user))
}

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

	Success(c, presentUsersForViewer(currentUser, users))
}

func (h *UserHandler) UpdateProfile(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	var input service.UpdateProfileInput
	if err := c.ShouldBindJSON(&input); err != nil {
		ErrorWithCode(c, http.StatusBadRequest, code.BadRequest, err.Error())
		return
	}

	user, err := h.service.UpdateProfile(currentUser, c.Param("uuid"), input)
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
		default:
			ErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
		}
		return
	}

	Success(c, presentUserForViewer(currentUser, user))
}

func (h *UserHandler) UpdateStatus(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	var input struct {
		Status int8 `json:"status"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		ErrorWithCode(c, http.StatusBadRequest, code.BadRequest, err.Error())
		return
	}

	user, err := h.service.UpdateStatus(currentUser, c.Param("uuid"), input.Status)
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

	Success(c, presentUserForViewer(currentUser, user))
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
