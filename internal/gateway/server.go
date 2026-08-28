package gateway

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/JekYUlll/Dipole/internal/application"
	httpHandler "github.com/JekYUlll/Dipole/internal/handler/http"
	"github.com/JekYUlll/Dipole/internal/logger"
	"github.com/JekYUlll/Dipole/internal/middleware"
	"github.com/JekYUlll/Dipole/internal/model"
	platformRateLimit "github.com/JekYUlll/Dipole/internal/platform/ratelimit"
	"github.com/JekYUlll/Dipole/internal/service"
	wsTransport "github.com/JekYUlll/Dipole/internal/transport/ws"
	"go.uber.org/zap"
)

type Dependencies struct {
	Messages        application.MessageApplication
	Core            application.CoreCapability
	Search          application.SearchApplication
	AgentTasks      AgentTaskControlApplication
	AgentMCP        AgentMCPApplication
	Presence        wsTransport.PresenceTracker
	Limiter         MessageRateLimiter
	AgentMCPLimiter AgentMCPRateLimiter
}

type MessageRateLimiter interface {
	AllowMessageSend(userUUID string) (bool, time.Duration)
}

type AgentMCPRateLimiter interface {
	AllowAgentMCP(principalUUID string) (bool, time.Duration)
}

type Server struct {
	engine     *gin.Engine
	wsHub      *wsTransport.Hub
	mu         sync.Mutex
	httpServer *http.Server
}

func NewServer(coreTarget string, dependencies Dependencies) (*Server, error) {
	if dependencies.Messages == nil {
		return nil, errors.New("gateway message application is required")
	}
	if dependencies.Core == nil {
		return nil, errors.New("gateway core capability is required")
	}
	target, err := url.Parse(coreTarget)
	if err != nil || (target.Scheme != "http" && target.Scheme != "https") || target.Host == "" {
		return nil, errors.New("gateway core http target is invalid")
	}

	engine := gin.New()
	engine.Use(middleware.Correlation(), logger.GinLogger(), logger.GinRecovery(), cors.Default())
	hub := wsTransport.NewHub(wsTransport.WithPresenceTracker(dependencies.Presence))
	tokenService := service.NewTokenService()
	userFinder := coreUserFinder{core: dependencies.Core}
	authenticator := wsTransport.NewAuthenticator(tokenService, userFinder)
	limiter := dependencies.Limiter
	if limiter == nil {
		limiter = platformRateLimit.NewLimiter()
	}
	dispatcher := wsTransport.NewDispatcher(hub, dependencies.Messages, nil, false).WithLimiter(limiter)
	wsHandler := wsTransport.NewHandler(authenticator, hub, dispatcher)

	engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "component": "gateway"})
	})
	engine.GET("/api/v1/ws", wsHandler.Handle)
	if dependencies.Search != nil {
		searchHandler := httpHandler.NewSearchHandler(dependencies.Search)
		engine.GET("/api/v1/messages/search", middleware.Auth(tokenService, userFinder), searchHandler.Search)
	}
	if dependencies.AgentTasks != nil {
		auth := middleware.Auth(tokenService, userFinder)
		engine.GET("/api/v1/agent/tasks/:task_id", auth, agentTaskGetHandler(dependencies.AgentTasks))
		engine.POST("/api/v1/agent/tasks/:task_id/cancel", auth, agentTaskCancelHandler(dependencies.AgentTasks))
		engine.POST("/api/v1/agent/tasks/:task_id/approvals/:approval_id", auth, agentTaskApprovalHandler(dependencies.AgentTasks))
		engine.POST("/api/v1/agent/tasks/:task_id/inputs/:request_id", auth, agentTaskInputHandler(dependencies.AgentTasks))
	}
	if dependencies.AgentMCP != nil {
		if err := service.ValidateAgentMCPResource(service.AgentMCPResourceIdentifier()); err != nil {
			return nil, errors.New("gateway Agent MCP resource is invalid")
		}
		auth := middleware.AgentMCPAuth(tokenService, userFinder)
		agentMCPLimiter := dependencies.AgentMCPLimiter
		if agentMCPLimiter == nil {
			agentMCPLimiter = platformRateLimit.NewLimiter()
		}
		engine.Any("/api/v1/agent/tasks/:task_id/runs/:run_id/mcp", auth, agentMCPHandler(dependencies.AgentMCP, agentMCPLimiter))
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(writer http.ResponseWriter, _ *http.Request, proxyErr error) {
		logger.Warn("gateway core proxy failed", zap.Error(proxyErr))
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusBadGateway)
		_, _ = writer.Write([]byte(`{"code":502,"message":"core service unavailable"}`))
	}
	engine.NoRoute(gin.WrapH(proxy))

	return &Server{engine: engine, wsHub: hub}, nil
}

func agentMCPHandler(proxy AgentMCPApplication, limiter AgentMCPRateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodPost && c.Request.Method != http.MethodDelete {
			c.Status(http.StatusMethodNotAllowed)
			return
		}
		user, ok := middleware.CurrentUser(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "user session is invalid"})
			return
		}
		if c.Request.Method != http.MethodDelete {
			allowed, retryAfter := limiter.AllowAgentMCP(user.UUID)
			if !allowed {
				seconds := int((retryAfter + time.Second - 1) / time.Second)
				if seconds < 1 {
					seconds = 1
				}
				c.Header("Retry-After", fmt.Sprintf("%d", seconds))
				c.JSON(http.StatusTooManyRequests, gin.H{"code": http.StatusTooManyRequests, "message": "Agent MCP rate limit exceeded"})
				return
			}
		}
		proxy.ServeMCP(c.Writer, c.Request, user.UUID, c.Param("task_id"), c.Param("run_id"))
	}
}

func agentTaskGetHandler(tasks AgentTaskControlApplication) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := middleware.CurrentUser(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "user session is invalid"})
			return
		}
		result, err := tasks.GetTask(c.Request.Context(), user.UUID, c.Param("task_id"))
		writeAgentTaskControlResult(c, result, err)
	}
}

func agentTaskCancelHandler(tasks AgentTaskControlApplication) gin.HandlerFunc {
	type requestBody struct {
		Reason string `json:"reason"`
	}
	return func(c *gin.Context) {
		user, ok := middleware.CurrentUser(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "user session is invalid"})
			return
		}
		var body requestBody
		if c.Request.ContentLength > 0 && c.ShouldBindJSON(&body) != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "invalid cancellation request"})
			return
		}
		result, err := tasks.CancelTask(c.Request.Context(), user.UUID, c.Param("task_id"), body.Reason)
		writeAgentTaskControlResult(c, result, err)
	}
}

func agentTaskApprovalHandler(tasks AgentTaskControlApplication) gin.HandlerFunc {
	type requestBody struct {
		Decision string `json:"decision"`
	}
	return func(c *gin.Context) {
		user, ok := middleware.CurrentUser(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "user session is invalid"})
			return
		}
		var body requestBody
		if c.ShouldBindJSON(&body) != nil || (body.Decision != "approved" && body.Decision != "denied") {
			c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "approval decision must be approved or denied"})
			return
		}
		result, err := tasks.ResolveApproval(c.Request.Context(), user.UUID, c.Param("task_id"), c.Param("approval_id"), body.Decision)
		writeAgentTaskControlResult(c, result, err)
	}
}

func agentTaskInputHandler(tasks AgentTaskControlApplication) gin.HandlerFunc {
	type requestBody struct {
		Value any `json:"value" binding:"required"`
	}
	return func(c *gin.Context) {
		user, ok := middleware.CurrentUser(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "user session is invalid"})
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32*1024)
		var body requestBody
		if c.ShouldBindJSON(&body) != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "invalid Agent Task input"})
			return
		}
		result, err := tasks.ProvideInput(c.Request.Context(), user.UUID, c.Param("task_id"), c.Param("request_id"), body.Value)
		writeAgentTaskControlResult(c, result, err)
	}
}

func writeAgentTaskControlResult(c *gin.Context, result *AgentTaskControlResult, err error) {
	if err != nil || result == nil {
		c.JSON(http.StatusBadGateway, gin.H{"code": http.StatusBadGateway, "message": "Agent Task control runtime unavailable"})
		return
	}
	contentType := result.ContentType
	if contentType == "" {
		contentType = "application/json; charset=utf-8"
	}
	c.Data(result.StatusCode, contentType, result.Body)
}

func (s *Server) Run(address string) error {
	return s.run(&http.Server{Addr: address, Handler: s.engine}, "", "")
}

func (s *Server) RunTLS(address, certFile, keyFile string) error {
	return s.run(&http.Server{Addr: address, Handler: s.engine}, certFile, keyFile)
}

func (s *Server) run(httpServer *http.Server, certFile, keyFile string) error {
	s.mu.Lock()
	s.httpServer = httpServer
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		if s.httpServer == httpServer {
			s.httpServer = nil
		}
		s.mu.Unlock()
	}()
	if certFile != "" || keyFile != "" {
		return httpServer.ListenAndServeTLS(certFile, keyFile)
	}
	return httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	httpServer := s.httpServer
	s.mu.Unlock()
	if httpServer != nil {
		if err := httpServer.Shutdown(ctx); err != nil {
			return err
		}
	}
	s.wsHub.CloseAll("gateway_shutdown")
	return nil
}

func (s *Server) Engine() *gin.Engine     { return s.engine }
func (s *Server) WSHub() *wsTransport.Hub { return s.wsHub }

type coreUserFinder struct {
	core application.CoreCapability
}

func (f coreUserFinder) GetByUUID(userUUID string) (*model.User, error) {
	return f.core.GetUserByUUID(userUUID)
}
