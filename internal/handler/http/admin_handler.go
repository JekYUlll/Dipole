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

type adminOverviewService interface {
	Overview(currentUser *model.User) (*service.AdminOverview, error)
}

type AdminHandler struct {
	service adminOverviewService
}

func NewAdminHandler(service adminOverviewService) *AdminHandler {
	return &AdminHandler{service: service}
}

func (h *AdminHandler) Overview(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}

	overview, err := h.service.Overview(currentUser)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAdminRequired):
			ErrorWithCode(c, http.StatusForbidden, code.UserAdminRequired, "admin permission is required")
		default:
			ErrorWithCode(c, http.StatusInternalServerError, code.Internal, err.Error())
		}
		return
	}

	Success(c, httpdto.ToAdminOverviewResponse(overview))
}
