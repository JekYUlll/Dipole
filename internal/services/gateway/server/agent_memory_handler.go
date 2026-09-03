package gateway

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/JekYUlll/Dipole/internal/middleware"
)

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

func agentMemoryCandidateListHandler(memories AgentMemoryControlApplication) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := middleware.CurrentUser(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "user session is invalid"})
			return
		}
		rawAfter, hasAfter := c.GetQuery("after")
		after := strings.TrimSpace(rawAfter)
		if hasAfter && (after != rawAfter || !validAgentSubscriptionPublicID(after, 72)) {
			c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "invalid Agent Memory candidate cursor"})
			return
		}
		limit := 50
		if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 1 || parsed > 100 {
				c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "Agent Memory candidate limit must be between 1 and 100"})
				return
			}
			limit = parsed
		}
		page, err := memories.ListCandidates(c.Request.Context(), user.UUID, after, limit)
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
		body.Content, body.CompactContent, body.Reason = strings.TrimSpace(body.Content), strings.TrimSpace(body.CompactContent), strings.TrimSpace(body.Reason)
		if body.ExpectedVersion == 0 || body.Content == "" || len([]byte(body.Content)) > 16*1024 || len([]byte(body.CompactContent)) > 4*1024 || body.Reason == "" || utf8.RuneCountInString(body.Reason) > 1000 || strings.IndexFunc(body.Reason, unicode.IsControl) >= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "Agent Memory correction request is invalid"})
			return
		}
		result, err := memories.Correct(c.Request.Context(), user.UUID, memoryID, body.ExpectedVersion, body.Content, body.CompactContent, body.Reason)
		writeAgentMemoryResult(c, result, err)
	}
}

func agentMemoryCandidatePromoteHandler(memories AgentMemoryControlApplication) gin.HandlerFunc {
	type requestBody struct {
		CandidateSHA256  string `json:"candidateSha256"`
		ReviewID         string `json:"reviewId"`
		TargetMemoryType string `json:"targetMemoryType"`
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
		body.CandidateSHA256, body.ReviewID, body.TargetMemoryType = strings.TrimSpace(body.CandidateSHA256), strings.TrimSpace(body.ReviewID), strings.TrimSpace(body.TargetMemoryType)
		if len(body.CandidateSHA256) != 64 || !isLowerHex(body.CandidateSHA256) || !validAgentSubscriptionPublicID(body.ReviewID, 72) || (body.TargetMemoryType != "" && !validPersistentAgentMemoryType(body.TargetMemoryType)) {
			c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "Agent Memory candidate promotion request is invalid"})
			return
		}
		item, err := memories.PromoteCandidate(c.Request.Context(), user.UUID, candidateID, body.CandidateSHA256, body.ReviewID, body.TargetMemoryType)
		writeAgentMemoryResult(c, item, err)
	}
}

func agentMemoryCandidateReviewHandler(memories AgentMemoryControlApplication) gin.HandlerFunc {
	type requestBody struct {
		CandidateSHA256 string `json:"candidateSha256"`
		Decision        string `json:"decision"`
		Reason          string `json:"reason"`
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
			c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "invalid Agent Memory candidate review request"})
			return
		}
		body.CandidateSHA256, body.Decision, body.Reason = strings.TrimSpace(body.CandidateSHA256), strings.TrimSpace(body.Decision), strings.TrimSpace(body.Reason)
		if len(body.CandidateSHA256) != 64 || !isLowerHex(body.CandidateSHA256) || (body.Decision != "accepted" && body.Decision != "rejected") || body.Reason == "" || utf8.RuneCountInString(body.Reason) > 1000 || strings.IndexFunc(body.Reason, unicode.IsControl) >= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "Agent Memory candidate review request is invalid"})
			return
		}
		item, err := memories.ReviewCandidate(c.Request.Context(), user.UUID, candidateID, body.CandidateSHA256, body.Decision, body.Reason)
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
