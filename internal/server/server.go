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
	wsTransport "github.com/JekYUlll/Dipole/internal/transport/ws"
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
	messageRepo := repository.NewMessageRepository()
	conversationRepo := repository.NewConversationRepository()
	contactRepo := repository.NewContactRepository()
	tokenService := service.NewTokenService()
	authService := service.NewAuthService(userRepo, tokenService)
	userService := service.NewUserService(userRepo)
	messageService := service.NewMessageService(messageRepo, userRepo, contactRepo)
	conversationService := service.NewConversationService(conversationRepo, userRepo)
	contactService := service.NewContactService(contactRepo, userRepo)
	wsHub := wsTransport.NewHub()
	wsAuthenticator := wsTransport.NewAuthenticator(tokenService, userRepo)
	wsDispatcher := wsTransport.NewDispatcher(wsHub, messageService, conversationService)
	authHandler := httpHandler.NewAuthHandler(authService)
	conversationHandler := httpHandler.NewConversationHandler(conversationService)
	contactHandler := httpHandler.NewContactHandler(contactService)
	userHandler := httpHandler.NewUserHandler(userService)
	messageHandler := httpHandler.NewMessageHandler(messageService)
	wsHandler := wsTransport.NewHandler(wsAuthenticator, wsHub, wsDispatcher)
	authRequired := middleware.Auth(tokenService, userRepo)

	v1 := engine.Group("/api/v1")
	{
		v1.GET("/ws", wsHandler.Handle)

		authGroup := v1.Group("/auth")
		{
			authGroup.POST("/register", authHandler.Register)
			authGroup.POST("/login", authHandler.Login)
		}

		protected := v1.Group("")
		protected.Use(authRequired)
		{
			protected.POST("/auth/logout", authHandler.Logout)
			protected.GET("/conversations", conversationHandler.List)
			protected.PATCH("/conversations/direct/:target_uuid/read", conversationHandler.MarkDirectRead)
			protected.GET("/contacts", contactHandler.ListFriends)
			protected.DELETE("/contacts/:friend_uuid", contactHandler.DeleteFriend)
			protected.POST("/contacts/applications", contactHandler.Apply)
			protected.GET("/contacts/applications", contactHandler.ListApplications)
			protected.PATCH("/contacts/applications/:id", contactHandler.HandleApplication)
			protected.GET("/messages/direct/:target_uuid", messageHandler.ListDirect)
			protected.GET("/users", userHandler.Search)
			protected.GET("/users/me", userHandler.GetCurrent)
			protected.GET("/users/:uuid", userHandler.GetByUUID)
			protected.PATCH("/users/:uuid/profile", userHandler.UpdateProfile)
			protected.GET("/admin/users", userHandler.ListForAdmin)
			protected.PATCH("/admin/users/:uuid/status", userHandler.UpdateStatus)
		}
	}

	return &Server{engine: engine}
}

func (s *Server) Run(addr string) error {
	return s.engine.Run(addr)
}

func (s *Server) RunTLS(addr, certFile, keyFile string) error {
	return s.engine.RunTLS(addr, certFile, keyFile)
}

func (s *Server) Engine() *gin.Engine {
	return s.engine
}
