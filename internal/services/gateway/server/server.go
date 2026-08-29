package gateway

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	httpHandler "github.com/JekYUlll/Dipole/internal/gateway/http"
	"github.com/JekYUlll/Dipole/internal/logger"
	"github.com/JekYUlll/Dipole/internal/middleware"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/platform/cache"
	platformRateLimit "github.com/JekYUlll/Dipole/internal/platform/ratelimit"
	coreauth "github.com/JekYUlll/Dipole/internal/services/core/domain/auth"
	wsTransport "github.com/JekYUlll/Dipole/internal/transport/ws"
	"go.uber.org/zap"
)

type Dependencies struct {
	Messages           application.MessageApplication
	Sync               application.SyncApplication
	Core               application.CoreCapability
	Search             application.SearchApplication
	AgentTasks         AgentTaskControlApplication
	AgentSubscriptions AgentSubscriptionControlApplication
	AgentDefinitions   AgentDefinitionCatalogApplication
	AgentMemories      AgentMemoryControlApplication
	AgentMCP           AgentMCPApplication
	Presence           wsTransport.PresenceTracker
	Limiter            MessageRateLimiter
	AgentMCPLimiter    AgentMCPRateLimiter
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
	tokenService := coreauth.NewTokenService()
	userFinder := coreUserFinder{core: dependencies.Core}
	authenticator := wsTransport.NewAuthenticator(tokenService, userFinder)
	limiter := dependencies.Limiter
	if limiter == nil {
		limiter = platformRateLimit.NewLimiterWithClient(config.RateLimitConfig(), cache.RDB)
	}
	dispatcher := wsTransport.NewDispatcher(hub, dependencies.Messages, nil, false).WithLimiter(limiter)
	wsHandler := wsTransport.NewHandler(authenticator, hub, dispatcher)
	auth := middleware.Auth(tokenService, userFinder)
	messageHandler := httpHandler.NewMessageHandler(dependencies.Messages)

	engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "component": "gateway"})
	})
	engine.GET("/api/v1/ws", wsHandler.Handle)
	protected := engine.Group("/api/v1")
	protected.Use(auth)
	protected.GET("/messages/offline", messageHandler.ListOffline)
	protected.GET("/messages/direct/:target_uuid", messageHandler.ListDirect)
	protected.GET("/messages/group/:group_uuid", messageHandler.ListGroup)
	if dependencies.Sync != nil {
		syncHandler := httpHandler.NewSyncHandler(dependencies.Sync)
		protected.GET("/sync", syncHandler.List)
		protected.GET("/sync/checkpoint", syncHandler.GetCheckpoint)
		protected.PATCH("/sync/checkpoint", syncHandler.AdvanceCheckpoint)
		protected.POST("/sync/comparison", syncHandler.ReportComparison)
		protected.GET("/sync/groups/checkpoints", syncHandler.ListGroupCheckpoints)
		protected.PATCH("/sync/groups/:group_uuid/checkpoint", syncHandler.AdvanceGroupCheckpoint)
	}
	if dependencies.Search != nil {
		searchHandler := NewSearchHandler(dependencies.Search)
		engine.GET("/api/v1/messages/search", middleware.Auth(tokenService, userFinder), searchHandler.Search)
	}
	if dependencies.AgentTasks != nil {
		engine.GET("/api/v1/agent/tasks/:task_id", auth, agentTaskGetHandler(dependencies.AgentTasks))
		engine.GET("/api/v1/agent/tasks/:task_id/timeline", auth, agentTaskTimelineHandler(dependencies.AgentTasks))
		engine.POST("/api/v1/agent/tasks/:task_id/cancel", auth, agentTaskCancelHandler(dependencies.AgentTasks))
		engine.POST("/api/v1/agent/tasks/:task_id/approvals/:approval_id", auth, agentTaskApprovalHandler(dependencies.AgentTasks))
		engine.POST("/api/v1/agent/tasks/:task_id/inputs/:request_id", auth, agentTaskInputHandler(dependencies.AgentTasks))
	}
	if dependencies.AgentSubscriptions != nil {
		engine.GET("/api/v1/agent/subscriptions", auth, agentSubscriptionListHandler(dependencies.AgentSubscriptions))
		engine.GET("/api/v1/agent/subscriptions/options", auth, agentSubscriptionConversationOptionsHandler(dependencies.AgentSubscriptions))
		engine.POST("/api/v1/agent/subscriptions", auth, agentSubscriptionCreateHandler(dependencies.AgentSubscriptions))
		engine.POST("/api/v1/agent/subscriptions/:subscription_id/revoke", auth, agentSubscriptionRevokeHandler(dependencies.AgentSubscriptions))
	}
	if dependencies.AgentDefinitions != nil {
		engine.GET("/api/v1/agent/definitions", auth, agentDefinitionCatalogHandler(dependencies.AgentDefinitions))
	}
	if dependencies.AgentMemories != nil {
		engine.GET("/api/v1/agent/memories", auth, agentMemoryListHandler(dependencies.AgentMemories))
		engine.POST("/api/v1/agent/memories/:memory_id/revoke", auth, agentMemoryRevokeHandler(dependencies.AgentMemories))
		engine.POST("/api/v1/agent/memories/:memory_id/correct", auth, agentMemoryCorrectHandler(dependencies.AgentMemories))
		engine.POST("/api/v1/agent/memory-candidates/:candidate_id/promote", auth, agentMemoryCandidatePromoteHandler(dependencies.AgentMemories))
	}
	if dependencies.AgentMCP != nil {
		if err := application.ValidateAgentMCPResource(application.AgentMCPResourceIdentifier(config.AuthConfig().AgentMCPResource)); err != nil {
			return nil, errors.New("gateway Agent MCP resource is invalid")
		}
		auth := middleware.AgentMCPAuth(tokenService, userFinder)
		agentMCPLimiter := dependencies.AgentMCPLimiter
		if agentMCPLimiter == nil {
			agentMCPLimiter = platformRateLimit.NewLimiterWithClient(config.RateLimitConfig(), cache.RDB)
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

func agentMemoryListHandler(memories AgentMemoryControlApplication) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := middleware.CurrentUser(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "user session is invalid"})
			return
		}
		rawAfter, hasAfter := c.GetQuery("after")
		after := strings.TrimSpace(rawAfter)
		if hasAfter && (after != rawAfter || !validAgentSubscriptionPublicID(after, 256)) {
			c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "invalid Agent Memory cursor"})
			return
		}
		limit := 50
		if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 1 || parsed > 100 {
				c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "Agent Memory limit must be between 1 and 100"})
				return
			}
			limit = parsed
		}
		page, err := memories.List(c.Request.Context(), user.UUID, after, limit)
		writeAgentMemoryResult(c, page, err)
	}
}

func agentMemoryRevokeHandler(memories AgentMemoryControlApplication) gin.HandlerFunc {
	type requestBody struct {
		Reason string `json:"reason"`
	}
	return func(c *gin.Context) {
		user, ok := middleware.CurrentUser(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "user session is invalid"})
			return
		}
		memoryID := strings.TrimSpace(c.Param("memory_id"))
		if !validAgentSubscriptionPublicID(memoryID, 64) {
			c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "invalid Agent Memory identity"})
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4*1024)
		var body requestBody
		if decodeStrictAgentSubscriptionBody(c.Request.Body, &body) != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "invalid Agent Memory revoke request"})
			return
		}
		body.Reason = strings.TrimSpace(body.Reason)
		if body.Reason == "" || utf8.RuneCountInString(body.Reason) > 1000 || strings.IndexFunc(body.Reason, unicode.IsControl) >= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "Agent Memory revoke reason is invalid"})
			return
		}
		item, err := memories.Revoke(c.Request.Context(), user.UUID, memoryID, body.Reason)
		writeAgentMemoryResult(c, item, err)
	}
}

func agentMemoryCorrectHandler(memories AgentMemoryControlApplication) gin.HandlerFunc {
	type requestBody struct {
		ExpectedVersion uint32 `json:"expectedVersion"`
		Content         string `json:"content"`
		CompactContent  string `json:"compactContent"`
		Reason          string `json:"reason"`
	}
	return func(c *gin.Context) {
		user, ok := middleware.CurrentUser(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "user session is invalid"})
			return
		}
		memoryID := strings.TrimSpace(c.Param("memory_id"))
		if !validAgentSubscriptionPublicID(memoryID, 64) {
			c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "invalid Agent Memory identity"})
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 24*1024)
		var body requestBody
		if decodeStrictAgentSubscriptionBody(c.Request.Body, &body) != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "invalid Agent Memory correction request"})
			return
		}
		body.Content = strings.TrimSpace(body.Content)
		body.CompactContent = strings.TrimSpace(body.CompactContent)
		body.Reason = strings.TrimSpace(body.Reason)
		if body.ExpectedVersion == 0 || body.Content == "" || len([]byte(body.Content)) > 16*1024 || len([]byte(body.CompactContent)) > 4*1024 ||
			body.Reason == "" || utf8.RuneCountInString(body.Reason) > 1000 || strings.IndexFunc(body.Reason, unicode.IsControl) >= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "Agent Memory correction request is invalid"})
			return
		}
		result, err := memories.Correct(c.Request.Context(), user.UUID, memoryID, body.ExpectedVersion, body.Content, body.CompactContent, body.Reason)
		writeAgentMemoryResult(c, result, err)
	}
}

func agentMemoryCandidatePromoteHandler(memories AgentMemoryControlApplication) gin.HandlerFunc {
	type requestBody struct {
		CandidateSHA256 string `json:"candidateSha256"`
		ReviewID        string `json:"reviewId"`
	}
	return func(c *gin.Context) {
		user, ok := middleware.CurrentUser(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "user session is invalid"})
			return
		}
		candidateID := strings.TrimSpace(c.Param("candidate_id"))
		if !validAgentSubscriptionPublicID(candidateID, 72) {
			c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "invalid Agent Memory candidate identity"})
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4*1024)
		var body requestBody
		if decodeStrictAgentSubscriptionBody(c.Request.Body, &body) != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "invalid Agent Memory candidate promotion request"})
			return
		}
		body.CandidateSHA256, body.ReviewID = strings.TrimSpace(body.CandidateSHA256), strings.TrimSpace(body.ReviewID)
		if len(body.CandidateSHA256) != 64 || !isLowerHex(body.CandidateSHA256) || !validAgentSubscriptionPublicID(body.ReviewID, 72) {
			c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "Agent Memory candidate promotion request is invalid"})
			return
		}
		item, err := memories.PromoteCandidate(c.Request.Context(), user.UUID, candidateID, body.CandidateSHA256, body.ReviewID)
		writeAgentMemoryResult(c, item, err)
	}
}

func writeAgentMemoryResult(c *gin.Context, value any, err error) {
	if err != nil || value == nil {
		statusCode := AgentMemoryHTTPStatus(err)
		message := "Agent Memory control is unavailable"
		switch statusCode {
		case http.StatusBadRequest:
			message = "Agent Memory request is invalid"
		case http.StatusForbidden:
			message = "Agent Memory access denied"
		case http.StatusConflict:
			message = "Agent Memory changed concurrently"
		}
		c.JSON(statusCode, gin.H{"code": statusCode, "message": message})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": value})
}

func AgentMemoryHTTPStatus(err error) int {
	switch {
	case errors.Is(err, ErrAgentMemoryInvalid):
		return http.StatusBadRequest
	case errors.Is(err, ErrAgentMemoryDenied):
		return http.StatusForbidden
	case errors.Is(err, ErrAgentMemoryConflict):
		return http.StatusConflict
	default:
		return http.StatusServiceUnavailable
	}
}

func agentSubscriptionConversationOptionsHandler(subscriptions AgentSubscriptionControlApplication) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := middleware.CurrentUser(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "user session is invalid"})
			return
		}
		definitionID := strings.TrimSpace(c.Query("definitionId"))
		version, err := strconv.ParseUint(strings.TrimSpace(c.Query("definitionVersion")), 10, 64)
		if !validAgentSubscriptionPublicID(definitionID, 64) || err != nil || version == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "invalid Agent Definition selection"})
			return
		}
		result, err := subscriptions.ListEligibleConversations(c.Request.Context(), user.UUID, definitionID, version)
		writeAgentSubscriptionResult(c, result, err)
	}
}

func agentSubscriptionCreateHandler(subscriptions AgentSubscriptionControlApplication) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := middleware.CurrentUser(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "user session is invalid"})
			return
		}
		var input AgentSubscriptionCreateInput
		if err := decodeStrictAgentSubscriptionBody(c.Request.Body, &input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "invalid Agent Subscription create payload"})
			return
		}
		item, err := subscriptions.Create(c.Request.Context(), user.UUID, input)
		writeAgentSubscriptionResult(c, item, err)
	}
}

func agentDefinitionCatalogHandler(catalog AgentDefinitionCatalogApplication) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := middleware.CurrentUser(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "user session is invalid"})
			return
		}
		rawAfter, hasAfter := c.GetQuery("after")
		after := strings.TrimSpace(rawAfter)
		if hasAfter && (after == "" || len(after) > 384) {
			c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "invalid Agent Definition cursor"})
			return
		}
		limit := 50
		if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 1 || parsed > 100 {
				c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "Agent Definition limit must be between 1 and 100"})
				return
			}
			limit = parsed
		}
		page, err := catalog.ListDefinitions(c.Request.Context(), user.UUID, after, limit)
		writeAgentSubscriptionResult(c, page, err)
	}
}

func agentSubscriptionListHandler(subscriptions AgentSubscriptionControlApplication) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := middleware.CurrentUser(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "user session is invalid"})
			return
		}
		rawAfter, hasAfter := c.GetQuery("after")
		after := strings.TrimSpace(rawAfter)
		if hasAfter && !validAgentSubscriptionPublicID(after, 64) {
			c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "invalid Agent Subscription cursor"})
			return
		}
		limit := 50
		if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 1 || parsed > 100 {
				c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "Agent Subscription limit must be between 1 and 100"})
				return
			}
			limit = parsed
		}
		page, err := subscriptions.List(c.Request.Context(), user.UUID, after, limit)
		writeAgentSubscriptionResult(c, page, err)
	}
}

func agentSubscriptionRevokeHandler(subscriptions AgentSubscriptionControlApplication) gin.HandlerFunc {
	type requestBody struct {
		Reason string `json:"reason" binding:"required"`
	}
	return func(c *gin.Context) {
		user, ok := middleware.CurrentUser(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "user session is invalid"})
			return
		}
		subscriptionID := strings.TrimSpace(c.Param("subscription_id"))
		if !validAgentSubscriptionPublicID(subscriptionID, 64) {
			c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "invalid Agent Subscription identity"})
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4*1024)
		var body requestBody
		if c.ShouldBindJSON(&body) != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "invalid Agent Subscription revoke request"})
			return
		}
		body.Reason = strings.TrimSpace(body.Reason)
		if body.Reason == "" || utf8.RuneCountInString(body.Reason) > 1000 || strings.IndexFunc(body.Reason, unicode.IsControl) >= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "Agent Subscription revoke reason is invalid"})
			return
		}
		item, err := subscriptions.Revoke(c.Request.Context(), user.UUID, subscriptionID, body.Reason)
		writeAgentSubscriptionResult(c, item, err)
	}
}

func validAgentSubscriptionPublicID(value string, maximum int) bool {
	if value == "" || len(value) > maximum {
		return false
	}
	for index, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') ||
			(index > 0 && (char == '_' || char == '.' || char == ':' || char == '-')) {
			continue
		}
		return false
	}
	return true
}

func writeAgentSubscriptionResult(c *gin.Context, value any, err error) {
	if err != nil || value == nil {
		statusCode := AgentSubscriptionHTTPStatus(err)
		message := "Agent Subscription control is unavailable"
		if statusCode == http.StatusBadRequest {
			message = "Agent Subscription request is invalid"
		} else if statusCode == http.StatusForbidden {
			message = "Agent Subscription access denied"
		} else if statusCode == http.StatusConflict {
			message = "Agent Subscription changed concurrently"
		}
		c.JSON(statusCode, gin.H{"code": statusCode, "message": message})
		return
	}
	c.JSON(http.StatusOK, value)
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

func agentTaskTimelineHandler(tasks AgentTaskControlApplication) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := middleware.CurrentUser(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "user session is invalid"})
			return
		}
		after := c.Query("after")
		if after != "" {
			if _, err := strconv.ParseUint(after, 10, 64); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "Agent Task Timeline cursor is invalid"})
				return
			}
		}
		limit := 50
		if raw := c.Query("limit"); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 1 || parsed > 100 {
				c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "Agent Task Timeline limit is invalid"})
				return
			}
			limit = parsed
		}
		result, err := tasks.GetTimeline(c.Request.Context(), user.UUID, c.Param("task_id"), after, limit)
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
