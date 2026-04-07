package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/repository"
	"github.com/JekYUlll/Dipole/internal/service"
)

const (
	ContextUserKey  = "currentUser"
	ContextTokenKey = "accessToken"
)

func Auth(tokenService *service.TokenService, userRepo *repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := parseBearerToken(c.GetHeader("Authorization"))
		if !ok {
			writeError(c, http.StatusUnauthorized, "authorization token is required")
			c.Abort()
			return
		}

		userUUID, err := tokenService.Resolve(token)
		if err != nil {
			writeError(c, http.StatusUnauthorized, "authorization token is invalid")
			c.Abort()
			return
		}

		user, err := userRepo.GetByUUID(userUUID)
		if err != nil {
			writeError(c, http.StatusInternalServerError, err.Error())
			c.Abort()
			return
		}
		if user == nil || user.Status == model.UserStatusDisabled {
			writeError(c, http.StatusUnauthorized, "user session is invalid")
			c.Abort()
			return
		}

		c.Set(ContextUserKey, user)
		c.Set(ContextTokenKey, token)
		c.Next()
	}
}

func CurrentUser(c *gin.Context) (*model.User, bool) {
	val, ok := c.Get(ContextUserKey)
	if !ok {
		return nil, false
	}

	user, ok := val.(*model.User)
	return user, ok
}

func CurrentToken(c *gin.Context) (string, bool) {
	val, ok := c.Get(ContextTokenKey)
	if !ok {
		return "", false
	}

	token, ok := val.(string)
	return token, ok
}

func parseBearerToken(header string) (string, bool) {
	header = strings.TrimSpace(header)
	if header == "" {
		return "", false
	}

	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}

	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", false
	}

	return token, true
}

func writeError(c *gin.Context, statusCode int, message string) {
	c.JSON(statusCode, gin.H{
		"code":    statusCode,
		"message": message,
	})
}
