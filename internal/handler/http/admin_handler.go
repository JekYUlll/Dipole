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

// Overview godoc
// @Summary 获取后台总览
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Success 200 {object} AdminOverviewResponseEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 403 {object} ErrorEnvelope
// @Failure 500 {object} ErrorEnvelope
// @Router /admin/overview [get]
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
