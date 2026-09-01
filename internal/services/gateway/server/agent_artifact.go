package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"regexp"
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
	Get(ctx context.Context, principalUUID, artifactID string) (*AgentArtifact, error)
	GetContent(ctx context.Context, principalUUID, artifactID string) (*AgentArtifactContent, error)
}

type agentArtifactRPC interface {
	GetArtifact(context.Context, *agentv1.GetArtifactRequest, ...grpc.CallOption) (*agentv1.GetArtifactResponse, error)
}

type AgentArtifactClient struct {
	rpc     agentArtifactRPC
	timeout time.Duration
}

type agentArtifactRPCResult struct {
	artifact *AgentArtifact
	content  []byte
}

func NewAgentArtifactClient(rpc agentArtifactRPC, timeout time.Duration) (*AgentArtifactClient, error) {
	if rpc == nil {
		return nil, ErrAgentArtifactInvalid
	}
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &AgentArtifactClient{rpc: rpc, timeout: timeout}, nil
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
