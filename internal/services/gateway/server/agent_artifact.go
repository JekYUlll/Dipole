package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	agentv1 "github.com/JekYUlll/Dipole/api/gen/go/agent/v1"
	"github.com/JekYUlll/Dipole/internal/middleware"
	grpccommon "github.com/JekYUlll/Dipole/internal/transport/grpc/common"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var agentArtifactIDPattern = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)

const (
	agentConversationDigestArtifactType = "conversation_digest"
	agentConversationDigestMediaType    = "text/markdown"
)

var (
	ErrAgentArtifactInvalid     = errors.New("Agent Artifact request is invalid")
	ErrAgentArtifactDenied      = errors.New("Agent Artifact access denied")
	ErrAgentArtifactUnavailable = errors.New("Agent Artifact storage is unavailable")
)

// AgentArtifact deliberately excludes content, object location and metadata JSON.
// A later, separately reviewed download flow must define its own disclosure policy.
type AgentArtifact struct {
	ArtifactID      string `json:"artifactId"`
	TaskID          string `json:"taskId"`
	RunID           string `json:"runId"`
	ArtifactType    string `json:"artifactType"`
	Version         uint32 `json:"version"`
	Title           string `json:"title"`
	MediaType       string `json:"mediaType"`
	ContentSHA256   string `json:"contentSha256"`
	SizeBytes       uint64 `json:"sizeBytes"`
	CreatedAtUnixMS int64  `json:"createdAtUnixMs"`
}

// AgentArtifactContent exposes the only user-renderable Artifact form. Object
// locations and Artifact metadata remain internal to the Core service.
type AgentArtifactContent struct {
	ArtifactID string `json:"artifactId"`
	MediaType  string `json:"mediaType"`
	Content    string `json:"content"`
}

type AgentArtifactApplication interface {
	List(context.Context, string, string, int) (*AgentArtifactPage, error)
	Get(ctx context.Context, principalUUID, artifactID string) (*AgentArtifact, error)
	GetContent(ctx context.Context, principalUUID, artifactID string) (*AgentArtifactContent, error)
}

type AgentArtifactPage struct {
	Artifacts  []AgentArtifact `json:"artifacts"`
	NextCursor string          `json:"nextCursor,omitempty"`
}

type agentArtifactRPC interface {
	ListOwnedArtifacts(context.Context, *agentv1.ListOwnedArtifactsRequest, ...grpc.CallOption) (*agentv1.ListOwnedArtifactsResponse, error)
	GetArtifact(context.Context, *agentv1.GetArtifactRequest, ...grpc.CallOption) (*agentv1.GetArtifactResponse, error)
}

func (c *AgentArtifactClient) List(ctx context.Context, principalUUID, after string, limit int) (*AgentArtifactPage, error) {
	principalUUID = strings.TrimSpace(principalUUID)
	afterCreatedAt, afterArtifactID, validCursor := parseAgentArtifactCursor(after)
	if principalUUID == "" || !validCursor || limit < 1 || limit > 100 {
		return nil, ErrAgentArtifactInvalid
	}
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	response, err := c.rpc.ListOwnedArtifacts(callCtx, &agentv1.ListOwnedArtifactsRequest{Context: grpccommon.RequestContextFrom(ctx, principalUUID, "dipole-gateway"), TenantId: c.tenantID, AfterCreatedAtUnixMs: afterCreatedAt, AfterArtifactId: afterArtifactID, Limit: uint32(limit)})
	if err != nil {
		return nil, mapAgentArtifactRPCError(err)
	}
	if response == nil || (response.GetNextCreatedAtUnixMs() == 0) != (response.GetNextArtifactId() == "") || (response.GetNextArtifactId() != "" && !agentArtifactIDPattern.MatchString(response.GetNextArtifactId())) {
		return nil, ErrAgentArtifactUnavailable
	}
	page := &AgentArtifactPage{Artifacts: make([]AgentArtifact, 0, len(response.GetArtifacts()))}
	if response.GetNextArtifactId() != "" {
		page.NextCursor = formatAgentArtifactCursor(response.GetNextCreatedAtUnixMs(), response.GetNextArtifactId())
	}
	for _, raw := range response.GetArtifacts() {
		if raw == nil || len(raw.GetMetadataJson()) != 0 || !validAgentArtifactMetadata(raw) {
			return nil, ErrAgentArtifactUnavailable
		}
		page.Artifacts = append(page.Artifacts, AgentArtifact{ArtifactID: raw.GetArtifactId(), TaskID: raw.GetTaskId(), RunID: raw.GetRunId(), ArtifactType: raw.GetArtifactType(), Version: raw.GetVersion(), Title: raw.GetTitle(), MediaType: raw.GetMediaType(), ContentSHA256: raw.GetContentSha256(), SizeBytes: raw.GetSizeBytes(), CreatedAtUnixMS: raw.GetCreatedAtUnixMs()})
	}
	return page, nil
}

func parseAgentArtifactCursor(cursor string) (int64, string, bool) {
	cursor = strings.TrimSpace(cursor)
	if cursor == "" {
		return 0, "", true
	}
	parts := strings.Split(cursor, ":")
	if len(parts) != 2 || parts[0] == "" || !agentArtifactIDPattern.MatchString(parts[1]) {
		return 0, "", false
	}
	createdAt, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || createdAt <= 0 {
		return 0, "", false
	}
	return createdAt, parts[1], true
}

func formatAgentArtifactCursor(createdAt int64, artifactID string) string {
	return strconv.FormatInt(createdAt, 10) + ":" + artifactID
}

func validAgentArtifactMetadata(item *agentv1.AgentArtifact) bool {
	return agentArtifactIDPattern.MatchString(item.GetArtifactId()) && item.GetTaskId() != "" && item.GetRunId() != "" && item.GetArtifactType() != "" && item.GetVersion() > 0 && item.GetTitle() != "" && item.GetMediaType() != "" && agentArtifactIDPattern.MatchString(item.GetContentSha256()) && item.GetSizeBytes() > 0 && item.GetCreatedAtUnixMs() > 0
}

func agentArtifactListHandler(artifacts AgentArtifactApplication) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := middleware.CurrentUser(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "user session is invalid"})
			return
		}
		limit := 50
		if raw := c.Query("limit"); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 1 || parsed > 100 {
				c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "invalid Agent Artifact limit"})
				return
			}
			limit = parsed
		}
		page, err := artifacts.List(c.Request.Context(), user.UUID, c.Query("after"), limit)
		if err != nil {
			code := agentArtifactHTTPStatus(err)
			c.JSON(code, gin.H{"code": code, "message": "Agent Artifact catalog is unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": http.StatusOK, "data": page})
	}
}

type AgentArtifactClient struct {
	rpc      agentArtifactRPC
	tenantID string
	timeout  time.Duration
}

type agentArtifactRPCResult struct {
	artifact *AgentArtifact
	content  []byte
}

func NewAgentArtifactClient(rpc agentArtifactRPC, tenantID string, timeout time.Duration) (*AgentArtifactClient, error) {
	tenantID = strings.TrimSpace(tenantID)
	if rpc == nil || tenantID == "" {
		return nil, ErrAgentArtifactInvalid
	}
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &AgentArtifactClient{rpc: rpc, tenantID: tenantID, timeout: timeout}, nil
}

func (c *AgentArtifactClient) Get(ctx context.Context, principalUUID, artifactID string) (*AgentArtifact, error) {
	result, err := c.get(ctx, principalUUID, artifactID)
	if err != nil {
		return nil, err
	}
	return result.artifact, nil
}

func (c *AgentArtifactClient) GetContent(ctx context.Context, principalUUID, artifactID string) (*AgentArtifactContent, error) {
	result, err := c.get(ctx, principalUUID, artifactID)
	if err != nil {
		return nil, err
	}
	if result.artifact.ArtifactType != agentConversationDigestArtifactType || result.artifact.MediaType != agentConversationDigestMediaType || !utf8.Valid(result.content) {
		return nil, ErrAgentArtifactDenied
	}
	return &AgentArtifactContent{ArtifactID: result.artifact.ArtifactID, MediaType: result.artifact.MediaType, Content: string(result.content)}, nil
}

func (c *AgentArtifactClient) get(ctx context.Context, principalUUID, artifactID string) (*agentArtifactRPCResult, error) {
	principalUUID, artifactID = strings.TrimSpace(principalUUID), strings.TrimSpace(artifactID)
	if principalUUID == "" || !agentArtifactIDPattern.MatchString(artifactID) {
		return nil, ErrAgentArtifactInvalid
	}
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	response, err := c.rpc.GetArtifact(callCtx, &agentv1.GetArtifactRequest{
		Context: grpccommon.RequestContextFrom(ctx, principalUUID, "dipole-gateway"), ArtifactId: artifactID,
	})
	if err != nil {
		return nil, mapAgentArtifactRPCError(err)
	}
	if response == nil || response.GetArtifact() == nil {
		return nil, ErrAgentArtifactUnavailable
	}
	raw := response.GetArtifact()
	if raw.GetArtifactId() != artifactID || raw.GetTaskId() == "" || raw.GetRunId() == "" || raw.GetArtifactType() == "" || raw.GetVersion() == 0 ||
		raw.GetTitle() == "" || raw.GetMediaType() == "" || raw.GetCreatedAtUnixMs() <= 0 || raw.GetContentSha256() == "" ||
		uint64(len(response.GetContent())) != raw.GetSizeBytes() {
		return nil, ErrAgentArtifactUnavailable
	}
	digest := sha256.Sum256(response.GetContent())
	if !strings.EqualFold(hex.EncodeToString(digest[:]), raw.GetContentSha256()) {
		return nil, ErrAgentArtifactUnavailable
	}
	return &agentArtifactRPCResult{artifact: &AgentArtifact{ArtifactID: raw.GetArtifactId(), TaskID: raw.GetTaskId(), RunID: raw.GetRunId(), ArtifactType: raw.GetArtifactType(),
		Version: raw.GetVersion(), Title: raw.GetTitle(), MediaType: raw.GetMediaType(), ContentSHA256: raw.GetContentSha256(), SizeBytes: raw.GetSizeBytes(), CreatedAtUnixMS: raw.GetCreatedAtUnixMs()}, content: response.GetContent()}, nil
}

func agentArtifactGetHandler(artifacts AgentArtifactApplication) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := middleware.CurrentUser(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "user session is invalid"})
			return
		}
		artifactID := strings.TrimSpace(c.Param("artifact_id"))
		if artifactID == "" || artifactID != c.Param("artifact_id") || !agentArtifactIDPattern.MatchString(artifactID) {
			c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "invalid Agent Artifact ID"})
			return
		}
		artifact, err := artifacts.Get(c.Request.Context(), user.UUID, artifactID)
		if err != nil {
			c.JSON(agentArtifactHTTPStatus(err), gin.H{"code": agentArtifactHTTPStatus(err), "message": "Agent Artifact is unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": http.StatusOK, "data": artifact})
	}
}

func agentArtifactContentHandler(artifacts AgentArtifactApplication) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := middleware.CurrentUser(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "user session is invalid"})
			return
		}
		artifactID := strings.TrimSpace(c.Param("artifact_id"))
		if artifactID == "" || artifactID != c.Param("artifact_id") || !agentArtifactIDPattern.MatchString(artifactID) {
			c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "invalid Agent Artifact ID"})
			return
		}
		artifact, err := artifacts.GetContent(c.Request.Context(), user.UUID, artifactID)
		if err != nil {
			c.JSON(agentArtifactHTTPStatus(err), gin.H{"code": agentArtifactHTTPStatus(err), "message": "Agent Artifact content is unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": http.StatusOK, "data": artifact})
	}
}

func mapAgentArtifactRPCError(err error) error {
	switch status.Code(err) {
	case codes.InvalidArgument:
		return ErrAgentArtifactInvalid
	case codes.PermissionDenied, codes.NotFound:
		return ErrAgentArtifactDenied
	default:
		return ErrAgentArtifactUnavailable
	}
}

func agentArtifactHTTPStatus(err error) int {
	switch {
	case errors.Is(err, ErrAgentArtifactInvalid):
		return http.StatusBadRequest
	case errors.Is(err, ErrAgentArtifactDenied):
		return http.StatusNotFound
	default:
		return http.StatusServiceUnavailable
	}
}
