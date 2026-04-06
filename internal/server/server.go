package server

import (
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/JekYUlll/Dipole/internal/config"
)

type Server struct {
	engine *gin.Engine
}

func New() *Server {
	engine := gin.New()
	engine.Use(gin.Logger(), gin.Recovery())
	engine.Use(cors.Default())

	appCfg := config.AppConfig()

	engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"app":    appCfg.Name,
			"env":    appCfg.Env,
		})
	})

	return &Server{engine: engine}
}

func (s *Server) Run(addr string) error {
	return s.engine.Run(addr)
}

func (s *Server) Engine() *gin.Engine {
	return s.engine
}
