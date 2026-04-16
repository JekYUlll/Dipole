package server

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed all:webapp
var webAppFiles embed.FS

func mountWebApp(engine *gin.Engine) {
	if engine == nil {
		return
	}

	subtree, err := fs.Sub(webAppFiles, "webapp")
	if err != nil {
		panic(err)
	}

	serveIndex := func(c *gin.Context) {
		content, readErr := fs.ReadFile(subtree, "index.html")
		if readErr != nil {
			c.Status(http.StatusNotFound)
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", content)
	}

	engine.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusTemporaryRedirect, "/app/")
	})
	engine.GET("/app", func(c *gin.Context) {
		c.Redirect(http.StatusTemporaryRedirect, "/app/")
	})
	engine.GET("/app/*filepath", func(c *gin.Context) {
		filePath := strings.TrimPrefix(c.Param("filepath"), "/")
		if filePath == "" {
			serveIndex(c)
			return
		}

		content, readErr := fs.ReadFile(subtree, filePath)
		if readErr != nil {
			// SPA fallback — let Vue Router handle the path
			serveIndex(c)
			return
		}

		contentType := mime.TypeByExtension(filepath.Ext(filePath))
		if contentType == "" {
			contentType = http.DetectContentType(content)
		}
		c.Data(http.StatusOK, contentType, content)
	})
}
