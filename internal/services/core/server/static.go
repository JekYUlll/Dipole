package server

import (
	"embed"
	"encoding/json"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed all:webapp
var webAppFiles embed.FS

// FrontendFlags controls which UI surfaces are visible at runtime.
// Each field maps to a VITE_AGENT_*_ENABLED compile-time flag that the
// SPA previously read from import.meta.env. When [FrontendFlags] is
// non-nil the Core server injects a <script> tag into index.html so the
// frontend can read the values from window.__DIPOLE_FLAGS__ instead,
// making the flags a deploy-time concern rather than a build-time one.
type FrontendFlags struct {
	Elicitation      bool `json:"elicitation"`
	Approval         bool `json:"approval"`
	Subscriptions    bool `json:"subscriptions"`
	Definitions      bool `json:"definitions"`
	Memories         bool `json:"memories"`
	MemoryCorrection bool `json:"memoryCorrection"`
	TaskCreate       bool `json:"taskCreate"`
	Timeline         bool `json:"timeline"`
	Artifacts        bool `json:"artifacts"`
}

// FrontendFlagsFromEnv reads DIPOLE_FRONTEND_AGENT_* environment variables
// and returns nil when none are set, preserving the compile-time default.
func FrontendFlagsFromEnv() *FrontendFlags {
	env := func(key string) bool { return strings.EqualFold(os.Getenv(key), "true") }
	flags := &FrontendFlags{
		Elicitation:      env("DIPOLE_FRONTEND_AGENT_ELICITATION_ENABLED"),
		Approval:         env("DIPOLE_FRONTEND_AGENT_APPROVAL_ENABLED"),
		Subscriptions:    env("DIPOLE_FRONTEND_AGENT_SUBSCRIPTIONS_ENABLED"),
		Definitions:      env("DIPOLE_FRONTEND_AGENT_DEFINITIONS_ENABLED"),
		Memories:         env("DIPOLE_FRONTEND_AGENT_MEMORIES_ENABLED"),
		MemoryCorrection: env("DIPOLE_FRONTEND_AGENT_MEMORY_CORRECTION_ENABLED"),
		TaskCreate:       env("DIPOLE_FRONTEND_AGENT_TASK_CREATE_ENABLED"),
		Timeline:         env("DIPOLE_FRONTEND_AGENT_TIMELINE_ENABLED"),
		Artifacts:        env("DIPOLE_FRONTEND_AGENT_ARTIFACTS_ENABLED"),
	}
	if *flags == (FrontendFlags{}) {
		return nil
	}
	return flags
}

const flagsPlaceholder = "</head>"

func buildFlagsSnippet(flags *FrontendFlags) string {
	if flags == nil {
		return ""
	}
	payload, err := json.Marshal(flags)
	if err != nil {
		return ""
	}
	return "<script>window.__DIPOLE_FLAGS__=" + string(payload) + "</script>"
}

func mountWebApp(engine *gin.Engine, flags *FrontendFlags) {
	if engine == nil {
		return
	}

	subtree, err := fs.Sub(webAppFiles, "webapp")
	if err != nil {
		panic(err)
	}

	snippet := buildFlagsSnippet(flags)

	serveIndex := func(c *gin.Context) {
		content, readErr := fs.ReadFile(subtree, "index.html")
		if readErr != nil {
			c.Status(http.StatusNotFound)
			return
		}
		html := string(content)
		if snippet != "" {
			html = strings.Replace(html, flagsPlaceholder, snippet+flagsPlaceholder, 1)
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
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
