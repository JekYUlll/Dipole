package gateway

import (
	"context"
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/logger"
	"github.com/JekYUlll/Dipole/internal/model"
	platformRateLimit "github.com/JekYUlll/Dipole/internal/platform/ratelimit"
	"github.com/JekYUlll/Dipole/internal/service"
	wsTransport "github.com/JekYUlll/Dipole/internal/transport/ws"
	"go.uber.org/zap"
)

type Dependencies struct {
	Messages application.MessageApplication
	Core     application.CoreCapability
	Presence wsTransport.PresenceTracker
	Limiter  MessageRateLimiter
}

type MessageRateLimiter interface {
	AllowMessageSend(userUUID string) (bool, time.Duration)
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
	engine.Use(logger.GinLogger(), logger.GinRecovery(), cors.Default())
	hub := wsTransport.NewHub(wsTransport.WithPresenceTracker(dependencies.Presence))
	authenticator := wsTransport.NewAuthenticator(service.NewTokenService(), coreUserFinder{core: dependencies.Core})
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
