package server

import (
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/JekYUlll/Dipole/internal/config"
	httpHandler "github.com/JekYUlll/Dipole/internal/handler/http"
	"github.com/JekYUlll/Dipole/internal/logger"
	"github.com/JekYUlll/Dipole/internal/middleware"
	"github.com/JekYUlll/Dipole/internal/repository"
	"github.com/JekYUlll/Dipole/internal/service"
)

type Server struct {
	engine *gin.Engine
}

func New() *Server {
	engine := gin.New()
	engine.Use(logger.GinLogger(), logger.GinRecovery())
	engine.Use(cors.Default())

	appCfg := config.AppConfig()

	engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"app":    appCfg.Name,
			"env":    appCfg.Env,
		})
	})

	userRepo := repository.NewUserRepository()
	tokenService := service.NewTokenService()
	authService := service.NewAuthService(userRepo, tokenService)
	userService := service.NewUserService(userRepo)
	authHandler := httpHandler.NewAuthHandler(authService)
	userHandler := httpHandler.NewUserHandler(userService)
	authRequired := middleware.Auth(tokenService, userRepo)

	v1 := engine.Group("/api/v1")
	{
		authGroup := v1.Group("/auth")
		{
			authGroup.POST("/register", authHandler.Register)
			authGroup.POST("/login", authHandler.Login)
		}

		protected := v1.Group("")
		protected.Use(authRequired)
		{
			protected.POST("/auth/logout", authHandler.Logout)
			protected.GET("/users/me", userHandler.GetCurrent)
			protected.GET("/users/:uuid", userHandler.GetByUUID)
			protected.PATCH("/users/:uuid/profile", userHandler.UpdateProfile)
		}
	}

	return &Server{engine: engine}
}

func (s *Server) Run(addr string) error {
	return s.engine.Run(addr)
}

func (s *Server) Engine() *gin.Engine {
	return s.engine
}
