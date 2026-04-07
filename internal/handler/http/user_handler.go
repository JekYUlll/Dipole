package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/JekYUlll/Dipole/internal/middleware"
	"github.com/JekYUlll/Dipole/internal/service"
)

type UserHandler struct {
	service *service.UserService
}

func NewUserHandler(service *service.UserService) *UserHandler {
	return &UserHandler{service: service}
}

func (h *UserHandler) GetCurrent(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		Error(c, http.StatusUnauthorized, "authorization token is required")
		return
	}

	Success(c, user)
}

func (h *UserHandler) GetByUUID(c *gin.Context) {
	user, err := h.service.GetByUUID(c.Param("uuid"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUserNotFound):
			Error(c, http.StatusNotFound, "user not found")
		default:
			Error(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	Success(c, user)
}
