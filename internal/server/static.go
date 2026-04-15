package server

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

//go:embed webapp/*
var webAppFiles embed.FS

func mountWebApp(engine *gin.Engine) {
	if engine == nil {
		return
	}

	subtree, err := fs.Sub(webAppFiles, "webapp")
	if err != nil {
		panic(err)
	}

	serveFile := func(c *gin.Context, name string) {
		content, readErr := fs.ReadFile(subtree, name)
		if readErr != nil {
			c.Status(http.StatusNotFound)
			return
		}

		contentType := mime.TypeByExtension(filepath.Ext(name))
		if contentType == "" {
			contentType = http.DetectContentType(content)
		}
		c.Data(http.StatusOK, contentType, content)
	}

	engine.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusTemporaryRedirect, "/app/")
	})
	engine.GET("/app", func(c *gin.Context) {
		c.Redirect(http.StatusTemporaryRedirect, "/app/")
	})
	engine.GET("/app/", func(c *gin.Context) {
		serveFile(c, "index.html")
	})
	engine.GET("/app/app.css", func(c *gin.Context) {
		serveFile(c, "app.css")
	})
	engine.GET("/app/app.js", func(c *gin.Context) {
		serveFile(c, "app.js")
	})
}
