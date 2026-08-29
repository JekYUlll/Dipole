package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/JekYUlll/Dipole/internal/code"
)

func Success(c *gin.Context, data any) {
	SuccessWithCode(c, code.Success, data)
}

func SuccessWithCode(c *gin.Context, bizCode int, data any) {
	c.JSON(http.StatusOK, gin.H{
		"code": bizCode,
		"data": data,
	})
}

func Error(c *gin.Context, statusCode int, message string) {
	ErrorWithCode(c, statusCode, statusCode, message)
}

func ErrorWithCode(c *gin.Context, statusCode int, bizCode int, message string) {
	c.JSON(statusCode, gin.H{
		"code":    bizCode,
		"message": message,
	})
}
