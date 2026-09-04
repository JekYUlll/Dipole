package server

import (
	"context"
	"net/http"
	"sync"
	"time"

	_ "github.com/JekYUlll/Dipole/docs/swagger"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	applicationPort "github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	httpHandler "github.com/JekYUlll/Dipole/internal/gateway/http"
	"github.com/JekYUlll/Dipole/internal/logger"
	"github.com/JekYUlll/Dipole/internal/middleware"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/platform/cache"
	platformHotGroup "github.com/JekYUlll/Dipole/internal/platform/hotgroup"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	platformPresence "github.com/JekYUlll/Dipole/internal/platform/presence"
	platformRateLimit "github.com/JekYUlll/Dipole/internal/platform/ratelimit"
	platformStorage "github.com/JekYUlll/Dipole/internal/platform/storage"
	coreapplication "github.com/JekYUlll/Dipole/internal/services/core/application"
	coreauth "github.com/JekYUlll/Dipole/internal/services/core/domain/auth"
	messageapplication "github.com/JekYUlll/Dipole/internal/services/message/application"
	wsTransport "github.com/JekYUlll/Dipole/internal/transport/ws"
)

type Server struct {
	engine     *gin.Engine
	wsHub      *wsTransport.Hub
	mu         sync.Mutex
	httpServer *http.Server
}

// Repositories contains only the stores required by the Core HTTP surface.
// Embedded composition adapts its aggregate repository set at the boundary.
type Repositories struct {
	Users         applicationPort.UserStore
	Files         applicationPort.FileMetadataStore
	Conversations applicationPort.ConversationStore
	Contacts      applicationPort.ContactStore
	Groups        applicationPort.GroupStore
	Admin         applicationPort.AdminOverviewStore
}

// MessagingServices is the minimum local/remote-compatible application set
// needed by the Core server. The aggregate composition remains outside this
// package and supplies these ports during embedded fallback.
type MessagingServices struct {
	Core          applicationPort.CoreCapability
	Files         *coreapplication.LocalFileApplication
	Messages      *messageapplication.LocalApplication
	Conversations *coreapplication.LocalConversationApplication
	Sync          applicationPort.SyncApplication
}

type Dependencies struct {
	Messages       applicationPort.MessageApplication
	Sync           applicationPort.SyncApplication
	SyncComparison applicationPort.ClientSyncComparisonObserver
	Messaging      *MessagingServices
	SystemMessages applicationPort.SystemMessageSender
	FrontendFlags  *FrontendFlags
}

func NewWithRepositories(repos *Repositories) *Server {
	return NewWithDependencies(repos, Dependencies{})
}

func NewWithDependencies(repos *Repositories, dependencies Dependencies) *Server {
	if repos == nil {
		panic("server repositories are required")
	}

	engine := gin.New()
	engine.Use(middleware.Correlation())
	engine.Use(logger.GinLogger(), logger.GinRecovery())
	engine.Use(cors.Default())
	mountWebApp(engine, dependencies.FrontendFlags)

	appCfg := config.AppConfig()

	engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"app":    appCfg.Name,
			"env":    appCfg.Env,
		})
	})
	engine.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	hotGroupDetector := platformHotGroup.NewDetectorWithClient(config.HotGroupConfig(), cache.RDB)
	redisPresence := platformPresence.NewRedisPresenceWithClient(config.PresenceConfig(), cache.RDB)
	wsHub := wsTransport.NewHub(wsTransport.WithPresenceTracker(wsTransport.NewRedisPresenceTracker(redisPresence)))
	requestLimiter := platformRateLimit.NewLimiterWithClient(config.RateLimitConfig(), cache.RDB)
	tokenService := coreauth.NewTokenService()
	authService := coreapplication.NewAuthApplication(repos.Users, tokenService)
	storageCfg := config.StorageConfig()
	userService := coreapplication.NewUserApplication(repos.Users, coreapplication.UserDependencies{
		Files: repos.Files, Storage: platformStorage.Client,
		AvatarMaxBytes: 5 * 1024 * 1024, AvatarURLTTL: 10 * time.Minute,
	})
	adminService := coreapplication.NewAdminApplication(repos.Admin, wsHub)
	var kafkaEvents applicationPort.EventPublisher
	if config.KafkaConfig().Enabled {
		kafkaEvents = platformKafka.Client
	}
	messaging := dependencies.Messaging
	if messaging == nil {
		messaging = &MessagingServices{}
	}
	if messaging.Conversations != nil {
		messaging.Conversations.WithNotifier(newConversationNotifier(wsHub))
	}
	messageApplication := applicationPort.MessageApplication(messaging.Messages)
	if dependencies.Messages != nil {
		messageApplication = dependencies.Messages
	}
	systemMessages := dependencies.SystemMessages
	if systemMessages == nil {
		systemMessages = messaging.Messages
	}
	contactService := coreapplication.NewContactApplication(repos.Contacts, repos.Users, coreapplication.ContactDependencies{
		Notifier: newContactNotifier(wsHub), Events: kafkaEvents, SystemMessenger: systemMessages,
	})
	groupService := coreapplication.NewGroupApplication(repos.Groups, repos.Users, coreapplication.GroupDependencies{
		Events: kafkaEvents, HotGroups: hotGroupDetector, Files: repos.Files,
		Storage: platformStorage.Client, AvatarMaxBytes: 5 * 1024 * 1024,
		AvatarURLTTL: 10 * time.Minute, SystemMessenger: systemMessages,
	})
	sessionService := coreapplication.NewSessionApplication(coreapplication.SessionDependencies{
		Presence: redisPresence, Tokens: tokenService,
		Kicker: newSessionKicker(wsHub, kafkaEvents, config.KafkaConfig().Enabled),
	})
	wsAuthenticator := wsTransport.NewAuthenticator(tokenService, repos.Users)
	// When Kafka is enabled, conversation updates are handled asynchronously by
	// updateDirectConversationHandler / updateGroupConversationHandler in bootstrap/embedded/kafka.go.
	// Passing nil here prevents the dispatcher from doing a redundant synchronous update.
	var conversationUpdater wsTransportConversationUpdater
	if !config.KafkaConfig().Enabled {
		conversationUpdater = messaging.Conversations
	}
	wsDispatcher := wsTransport.NewDispatcher(wsHub, messageApplication, conversationUpdater, !config.KafkaConfig().Enabled).
		WithTimelineNotifyMode(config.MessageConfig().TimelineNotifyMode).
		WithLimiter(requestLimiter)
	authHandler := httpHandler.NewAuthHandler(authService).WithLimiter(requestLimiter)
	adminHandler := httpHandler.NewAdminHandler(adminService)
	conversationHandler := httpHandler.NewConversationHandler(messaging.Conversations)
	contactHandler := httpHandler.NewContactHandler(contactService)
	groupHandler := httpHandler.NewGroupHandler(groupService).WithAvatarMaxUploadBytes(minInt64(5*1024*1024, storageCfg.FileMaxSizeMB*1024*1024))
	sessionHandler := httpHandler.NewSessionHandler(sessionService)
	userHandler := httpHandler.NewUserHandler(userService).WithAvatarMaxUploadBytes(minInt64(5*1024*1024, storageCfg.FileMaxSizeMB*1024*1024))
	messageHandler := httpHandler.NewMessageHandler(messageApplication)
	syncApplication := applicationPort.SyncApplication(messaging.Sync)
	if dependencies.Sync != nil {
		syncApplication = dependencies.Sync
	}
	syncHandler := httpHandler.NewSyncHandler(syncApplication).WithComparisonObserver(dependencies.SyncComparison)
	fileHandler := httpHandler.NewFileHandler(messaging.Files).WithDirectory(messaging.Core).WithLimiter(requestLimiter)
	wsHandler := wsTransport.NewHandler(wsAuthenticator, wsHub, wsDispatcher)
	authRequired := middleware.Auth(tokenService, repos.Users)

	v1 := engine.Group("/api/v1")
	{
		if coreOwnsHTTPDataRoutes(config.GatewayConfig().Mode) {
			v1.GET("/ws", wsHandler.Handle)
		}

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
			protected.PATCH("/auth/password", authHandler.ChangePassword)
			protected.POST("/auth/agent-mcp/token", authHandler.IssueAgentMCPGrant)
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
			if coreOwnsHTTPDataRoutes(config.GatewayConfig().Mode) {
				protected.GET("/messages/offline", messageHandler.ListOffline)
				protected.GET("/messages/direct/:target_uuid", messageHandler.ListDirect)
				protected.GET("/messages/group/:group_uuid", messageHandler.ListGroup)
				protected.GET("/sync", syncHandler.List)
				protected.GET("/sync/checkpoint", syncHandler.GetCheckpoint)
				protected.PATCH("/sync/checkpoint", syncHandler.AdvanceCheckpoint)
				protected.POST("/sync/comparison", syncHandler.ReportComparison)
				protected.GET("/sync/groups/checkpoints", syncHandler.ListGroupCheckpoints)
				protected.PATCH("/sync/groups/:group_uuid/checkpoint", syncHandler.AdvanceGroupCheckpoint)
			}
			protected.GET("/files", fileHandler.ListOwned)
			protected.POST("/files", fileHandler.Upload)
			protected.GET("/files/uploads/policy", fileHandler.MultipartPolicy)
			protected.POST("/files/uploads/initiate", fileHandler.InitiateMultipart)
			protected.GET("/files/uploads/:session_id", fileHandler.MultipartStatus)
			protected.POST("/files/uploads/:session_id/parts/presign", fileHandler.PresignMultipartParts)
			protected.POST("/files/uploads/:session_id/parts/:part_number/register", fileHandler.RegisterMultipartPart)
			protected.PUT("/files/uploads/:session_id/parts/:part_number", fileHandler.UploadPart)
			protected.POST("/files/uploads/:session_id/complete", fileHandler.CompleteMultipart)
			protected.DELETE("/files/uploads/:session_id", fileHandler.AbortMultipart)
			protected.GET("/files/:file_id/download", fileHandler.Download)
			protected.GET("/files/:file_id/content", fileHandler.Content)
			protected.GET("/users/me/devices", sessionHandler.ListDevices)
			protected.POST("/users/me/devices/:connection_id/logout", sessionHandler.ForceLogoutDevice)
			protected.POST("/users/me/devices/logout-all", sessionHandler.ForceLogoutAll)
			protected.POST("/users/me/devices/logout-others", sessionHandler.ForceLogoutOther)
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

func coreOwnsHTTPDataRoutes(gatewayMode string) bool {
	return gatewayMode == "embedded"
}

type wsTransportConversationUpdater interface {
	UpdateDirectConversations(message *model.Message) error
	UpdateGroupConversations(message *model.Message) error
}

func (s *Server) Run(addr string) error {
	server := &http.Server{
		Addr:    addr,
		Handler: s.engine,
	}
	s.setHTTPServer(server)
	defer s.clearHTTPServer(server)

	return server.ListenAndServe()
}

func (s *Server) RunTLS(addr, certFile, keyFile string) error {
	server := &http.Server{
		Addr:    addr,
		Handler: s.engine,
	}
	s.setHTTPServer(server)
	defer s.clearHTTPServer(server)

	return server.ListenAndServeTLS(certFile, keyFile)
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil {
		return nil
	}

	server := s.currentHTTPServer()
	if server != nil {
		if err := server.Shutdown(ctx); err != nil {
			return err
		}
	}

	if s.wsHub != nil {
		s.wsHub.CloseAll("server_shutdown")
	}

	return nil
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

func (s *Server) setHTTPServer(server *http.Server) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.httpServer = server
}

func (s *Server) clearHTTPServer(server *http.Server) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.httpServer == server {
		s.httpServer = nil
	}
}

func (s *Server) currentHTTPServer() *http.Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.httpServer
}
