package gateway

import (
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/code"
	"github.com/JekYUlll/Dipole/internal/dto/httpdto"
	httpHandler "github.com/JekYUlll/Dipole/internal/gateway/http"
	"github.com/JekYUlll/Dipole/internal/middleware"
)

type SearchHandler struct {
	service application.SearchApplication
}

func NewSearchHandler(service application.SearchApplication) *SearchHandler {
	return &SearchHandler{service: service}
}

// Search godoc
// @Summary 搜索当前用户可访问的消息
// @Tags Message
// @Security BearerAuth
// @Produce json
// @Param q query string true "检索文本，1..256 个字符"
// @Param limit query int false "返回数量，1..100"
// @Success 200 {object} SearchMessageListResponseEnvelope
// @Failure 400 {object} ErrorEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 502 {object} ErrorEnvelope
// @Router /messages/search [get]
func (h *SearchHandler) Search(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		httpHandler.ErrorWithCode(c, http.StatusUnauthorized, code.AuthTokenRequired, "authorization token is required")
		return
	}
	query := strings.TrimSpace(c.Query("q"))
	if query == "" || utf8.RuneCountInString(query) > 256 {
		httpHandler.ErrorWithCode(c, http.StatusBadRequest, code.BadRequest, "q must contain between 1 and 256 characters")
		return
	}
	limit := 20
	if rawLimit, present := c.GetQuery("limit"); present {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed < 1 || parsed > 100 {
			httpHandler.ErrorWithCode(c, http.StatusBadRequest, code.BadRequest, "limit must be between 1 and 100")
			return
		}
		limit = parsed
	}
	documents, err := h.service.Search(currentUser.UUID, query, limit)
	if err != nil {
		httpHandler.ErrorWithCode(c, http.StatusBadGateway, code.Internal, "search service unavailable")
		return
	}
	httpHandler.Success(c, httpdto.ToSearchMessageResponses(documents))
}
