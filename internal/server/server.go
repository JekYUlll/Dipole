package server

import (
	"context"
	"net/http"
	"time"

	_ "github.com/JekYUlll/Dipole/docs/swagger"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/JekYUlll/Dipole/internal/config"
	httpHandler "github.com/JekYUlll/Dipole/internal/handler/http"
	"github.com/JekYUlll/Dipole/internal/logger"
	"github.com/JekYUlll/Dipole/internal/middleware"
	"github.com/JekYUlll/Dipole/internal/model"
	platformHotGroup "github.com/JekYUlll/Dipole/internal/platform/hotgroup"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	platformPresence "github.com/JekYUlll/Dipole/internal/platform/presence"
	platformRateLimit "github.com/JekYUlll/Dipole/internal/platform/ratelimit"
	platformStorage "github.com/JekYUlll/Dipole/internal/platform/storage"
	"github.com/JekYUlll/Dipole/internal/repository"
	"github.com/JekYUlll/Dipole/internal/service"
	wsTransport "github.com/JekYUlll/Dipole/internal/transport/ws"
)

type Server struct {
	engine *gin.Engine
	wsHub  *wsTransport.Hub
}

type serverEventPublisher interface {
	PublishJSON(ctx context.Context, topic string, key string, payload any, headers map[string]string) error
	PublishEvent(ctx context.Context, topic string, key string, eventType string, payload any, headers map[string]string) error
}

func New() *Server {
	engine := gin.New()
	engine.Use(logger.GinLogger(), logger.GinRecovery())
	engine.Use(cors.Default())
	mountWebApp(engine)

	appCfg := config.AppConfig()

	engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"app":    appCfg.Name,
			"env":    appCfg.Env,
		})
	})
	engine.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	userRepo := repository.NewUserRepository()
	messageRepo := repository.NewMessageRepository()
	fileRepo := repository.NewFileRepository()
	conversationRepo := repository.NewConversationRepository()
	contactRepo := repository.NewContactRepository()
	groupRepo := repository.NewGroupRepository()
	adminRepo := repository.NewAdminRepository()
	hotGroupDetector := platformHotGroup.NewRedisDetector()
	redisPresence := platformPresence.NewRedisPresence()
	wsHub := wsTransport.NewHub(wsTransport.WithPresenceTracker(newWSPresenceTrackerAdapter(redisPresence)))
	requestLimiter := platformRateLimit.NewLimiter()
	tokenService := service.NewTokenService()
	authService := service.NewAuthService(userRepo, tokenService)
	storageCfg := config.StorageConfig()
	userService := service.NewUserService(userRepo).WithAvatarStorage(
		fileRepo,
		platformStorage.Client,
		5*1024*1024,
		10*time.Minute,
	)
	fileService := service.NewFileService(fileRepo, messageRepo, platformStorage.Client)
	adminService := service.NewAdminService(adminRepo, wsHub)
	var kafkaEvents serverEventPublisher
	if config.KafkaConfig().Enabled {
		kafkaEvents = platformKafka.Client
	}
	messageService := service.NewMessageService(messageRepo, userRepo, contactRepo, groupRepo, fileService, kafkaEvents, hotGroupDetector)
	conversationService := service.NewConversationService(conversationRepo, userRepo, groupRepo, newConversationNotifier(wsHub), kafkaEvents)
	contactService := service.NewContactService(contactRepo, userRepo).WithNotifier(newContactNotifier(wsHub)).WithEvents(kafkaEvents)
	groupService := service.NewGroupService(groupRepo, userRepo, kafkaEvents, hotGroupDetector).WithAvatarStorage(
		fileRepo,
		platformStorage.Client,
		5*1024*1024,
		10*time.Minute,
	)
	sessionService := service.NewSessionService(redisPresence, tokenService, newSessionKicker(wsHub, kafkaEvents, config.KafkaConfig().Enabled))
	wsAuthenticator := wsTransport.NewAuthenticator(tokenService, userRepo)
	// When Kafka is enabled, conversation updates are handled asynchronously by
	// updateDirectConversationHandler / updateGroupConversationHandler in bootstrap/kafka.go.
	// Passing nil here prevents the dispatcher from doing a redundant synchronous update.
	var conversationUpdater wsTransportConversationUpdater
	if !config.KafkaConfig().Enabled {
		conversationUpdater = conversationService
	}
	wsDispatcher := wsTransport.NewDispatcher(wsHub, messageService, conversationUpdater, !config.KafkaConfig().Enabled).WithLimiter(requestLimiter)
	authHandler := httpHandler.NewAuthHandler(authService).WithLimiter(requestLimiter)
	adminHandler := httpHandler.NewAdminHandler(adminService)
	conversationHandler := httpHandler.NewConversationHandler(conversationService)
	contactHandler := httpHandler.NewContactHandler(contactService)
	groupHandler := httpHandler.NewGroupHandler(groupService).WithAvatarMaxUploadBytes(minInt64(5*1024*1024, storageCfg.FileMaxSizeMB*1024*1024))
	sessionHandler := httpHandler.NewSessionHandler(sessionService)
	userHandler := httpHandler.NewUserHandler(userService).WithAvatarMaxUploadBytes(minInt64(5*1024*1024, storageCfg.FileMaxSizeMB*1024*1024))
	messageHandler := httpHandler.NewMessageHandler(messageService)
	fileHandler := httpHandler.NewFileHandler(fileService).WithLimiter(requestLimiter)
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

		v1.GET("/users/:uuid/avatar", userHandler.GetAvatar)
		v1.GET("/groups/:uuid/avatar", groupHandler.GetAvatar)

		protected := v1.Group("")
		protected.Use(authRequired)
		{
			protected.POST("/auth/logout", authHandler.Logout)
			protected.GET("/conversations", conversationHandler.List)
			protected.PATCH("/conversations/direct/:target_uuid/read", conversationHandler.MarkDirectRead)
			protected.PATCH("/conversations/group/:group_uuid/read", conversationHandler.MarkGroupRead)
			protected.PATCH("/conversations/group/:group_uuid/remark", conversationHandler.UpdateGroupRemark)
			protected.GET("/contacts", contactHandler.ListFriends)
			protected.DELETE("/contacts/:friend_uuid", contactHandler.DeleteFriend)
			protected.PATCH("/contacts/:friend_uuid/remark", contactHandler.UpdateRemark)
			protected.PATCH("/contacts/:friend_uuid/block", contactHandler.UpdateBlockStatus)
			protected.POST("/contacts/applications", contactHandler.Apply)
			protected.GET("/contacts/applications", contactHandler.ListApplications)
			protected.PATCH("/contacts/applications/:id", contactHandler.HandleApplication)
			protected.POST("/groups", groupHandler.Create)
			protected.GET("/groups/:uuid", groupHandler.Get)
			protected.GET("/groups/:uuid/members", groupHandler.ListMembers)
			protected.POST("/groups/:uuid/members", groupHandler.AddMembers)
			protected.POST("/groups/:uuid/update", groupHandler.Update)
			protected.POST("/groups/:uuid/avatar", groupHandler.UploadAvatar)
			protected.POST("/groups/:uuid/remove-members", groupHandler.RemoveMembers)
			protected.POST("/groups/:uuid/dismiss", groupHandler.Dismiss)
			protected.DELETE("/groups/:uuid/members/me", groupHandler.Leave)
			protected.GET("/messages/offline", messageHandler.ListOffline)
			protected.GET("/messages/direct/:target_uuid", messageHandler.ListDirect)
			protected.GET("/messages/group/:group_uuid", messageHandler.ListGroup)
			protected.POST("/files", fileHandler.Upload)
			protected.POST("/files/uploads/initiate", fileHandler.InitiateMultipart)
			protected.PUT("/files/uploads/:session_id/parts/:part_number", fileHandler.UploadPart)
			protected.POST("/files/uploads/:session_id/complete", fileHandler.CompleteMultipart)
			protected.DELETE("/files/uploads/:session_id", fileHandler.AbortMultipart)
			protected.GET("/files/:file_id/download", fileHandler.Download)
			protected.GET("/files/:file_id/content", fileHandler.Content)
			protected.GET("/users/me/devices", sessionHandler.ListDevices)
			protected.POST("/users/me/devices/:connection_id/logout", sessionHandler.ForceLogoutDevice)
			protected.POST("/users/me/devices/logout-all", sessionHandler.ForceLogoutAll)
			protected.GET("/users", userHandler.Search)
			protected.GET("/users/me", userHandler.GetCurrent)
			protected.GET("/users/:uuid", userHandler.GetByUUID)
			protected.PATCH("/users/:uuid/profile", userHandler.UpdateProfile)
			protected.POST("/users/:uuid/avatar", userHandler.UploadAvatar)
			protected.GET("/admin/users", userHandler.ListForAdmin)
			protected.PATCH("/admin/users/:uuid/status", userHandler.UpdateStatus)
			protected.GET("/admin/overview", adminHandler.Overview)
		}
	}

	return &Server{engine: engine, wsHub: wsHub}
}

type wsTransportConversationUpdater interface {
	UpdateDirectConversations(message *model.Message) error
	UpdateGroupConversations(message *model.Message) error
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

func (s *Server) WSHub() *wsTransport.Hub {
	if s == nil {
		return nil
	}

	return s.wsHub
}

func minInt64(a, b int64) int64 {
	switch {
	case a <= 0:
		return b
	case b <= 0:
		return a
	case a < b:
		return a
	default:
		return b
	}
}
