package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	agentv1 "github.com/JekYUlll/Dipole/api/gen/go/agent/v1"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/platform/cache"
	coreauth "github.com/JekYUlll/Dipole/internal/services/core/domain/auth"
	"google.golang.org/grpc"
)

type gatewayAgentArtifactRPCStub struct {
	request     *agentv1.GetArtifactRequest
	listRequest *agentv1.ListOwnedArtifactsRequest
	response    *agentv1.GetArtifactResponse
	list        *agentv1.ListOwnedArtifactsResponse
}

func (s *gatewayAgentArtifactRPCStub) ListOwnedArtifacts(_ context.Context, request *agentv1.ListOwnedArtifactsRequest, _ ...grpc.CallOption) (*agentv1.ListOwnedArtifactsResponse, error) {
	s.listRequest = request
	return s.list, nil
}

func (s *gatewayAgentArtifactRPCStub) GetArtifact(_ context.Context, request *agentv1.GetArtifactRequest, _ ...grpc.CallOption) (*agentv1.GetArtifactResponse, error) {
	s.request = request
	return s.response, nil
}

func artifactResponse(body []byte) *agentv1.GetArtifactResponse {
	digest := sha256.Sum256(body)
	return &agentv1.GetArtifactResponse{Artifact: &agentv1.AgentArtifact{
		SchemaVersion: "dipole.agent.artifact.v1", ArtifactId: strings.Repeat("a", 64), TaskId: "TASK-1", RunId: "RUN-1",
		ArtifactType: "conversation_digest", Version: 1, Title: "Daily digest", MediaType: "text/markdown",
		ContentSha256: hex.EncodeToString(digest[:]), SizeBytes: uint64(len(body)), MetadataJson: []byte(`{"private":"detail"}`), CreatedAtUnixMs: 1_700_000_000_000,
	}, Content: body}
}

func TestAgentArtifactClientBindsPrincipalAndLimitsContentToConversationDigests(t *testing.T) {
	rpc := &gatewayAgentArtifactRPCStub{response: artifactResponse([]byte("private artifact body"))}
	client, err := NewAgentArtifactClient(rpc, "dipole", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := client.Get(context.Background(), "U100", strings.Repeat("a", 64))
	if err != nil || artifact.ArtifactID != strings.Repeat("a", 64) || artifact.Title != "Daily digest" || rpc.request.GetContext().GetPrincipalUserId() != "U100" || rpc.request.GetContext().GetCallerService() != "dipole-gateway" {
		t.Fatalf("artifact=%+v request=%+v err=%v", artifact, rpc.request, err)
	}
	if _, err := client.Get(context.Background(), "", strings.Repeat("a", 64)); err != ErrAgentArtifactInvalid {
		t.Fatalf("empty principal error=%v", err)
	}
	if _, err := client.Get(context.Background(), "U100", "ART-1"); err != ErrAgentArtifactInvalid {
		t.Fatalf("non-digest artifact ID error=%v", err)
	}
	content, err := client.GetContent(context.Background(), "U100", strings.Repeat("a", 64))
	if err != nil || content.Content != "private artifact body" || content.MediaType != "text/markdown" {
		t.Fatalf("content=%+v err=%v", content, err)
	}
	rpc.response.Artifact.ArtifactType = "report"
	if _, err := client.GetContent(context.Background(), "U100", strings.Repeat("a", 64)); err != ErrAgentArtifactDenied {
		t.Fatalf("non-digest content error=%v", err)
	}
	rpc.response.Artifact.ArtifactType = "conversation_digest"
	rpc.response.Artifact.ContentSha256 = strings.Repeat("0", 64)
	if _, err := client.Get(context.Background(), "U100", strings.Repeat("a", 64)); err != ErrAgentArtifactUnavailable {
		t.Fatalf("forged hash error=%v", err)
	}
}

func TestAgentArtifactClientListsOnlyOwnerMetadata(t *testing.T) {
	artifactID := strings.Repeat("a", 64)
	nextID := strings.Repeat("b", 64)
	rpc := &gatewayAgentArtifactRPCStub{list: &agentv1.ListOwnedArtifactsResponse{Artifacts: []*agentv1.AgentArtifact{{
		ArtifactId: artifactID, TaskId: "TASK-1", RunId: "RUN-1", ArtifactType: "conversation_digest", Version: 1, Title: "Daily digest", MediaType: "text/markdown", ContentSha256: strings.Repeat("c", 64), SizeBytes: 12, CreatedAtUnixMs: 1_700_000_000_000,
	}}, NextCreatedAtUnixMs: 1_699_000_000_000, NextArtifactId: nextID}}
	client, err := NewAgentArtifactClient(rpc, "tenant-a", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	page, err := client.List(context.Background(), "U100", "", 20)
	if err != nil || len(page.Artifacts) != 1 || page.NextCursor != "1699000000000:"+nextID || rpc.listRequest.GetTenantId() != "tenant-a" || rpc.listRequest.GetContext().GetPrincipalUserId() != "U100" {
		t.Fatalf("page=%+v request=%+v err=%v", page, rpc.listRequest, err)
	}
	_, err = client.List(context.Background(), "U100", page.NextCursor, 20)
	if err != nil || rpc.listRequest.GetAfterCreatedAtUnixMs() != 1_699_000_000_000 || rpc.listRequest.GetAfterArtifactId() != nextID {
		t.Fatalf("cursor request=%+v err=%v", rpc.listRequest, err)
	}
	rpc.list.Artifacts[0].MetadataJson = []byte(`{"private":"detail"}`)
	if _, err := client.List(context.Background(), "U100", "", 20); err != ErrAgentArtifactUnavailable {
		t.Fatalf("metadata leakage error=%v", err)
	}
}

type gatewayAgentArtifactStub struct {
	principal string
	artifact  string
	calls     int
}

func (s *gatewayAgentArtifactStub) List(_ context.Context, principalUUID, _ string, _ int) (*AgentArtifactPage, error) {
	s.principal, s.calls = principalUUID, s.calls+1
	return &AgentArtifactPage{}, nil
}

func (s *gatewayAgentArtifactStub) Get(_ context.Context, principalUUID, artifactID string) (*AgentArtifact, error) {
	s.principal, s.artifact, s.calls = principalUUID, artifactID, s.calls+1
	return &AgentArtifact{ArtifactID: artifactID, TaskID: "TASK-1", RunID: "RUN-1", ArtifactType: "conversation_digest", Version: 1, Title: "Digest", MediaType: "text/markdown", ContentSHA256: strings.Repeat("b", 64), SizeBytes: 12, CreatedAtUnixMS: 1_700_000_000_000}, nil
}

func (s *gatewayAgentArtifactStub) GetContent(_ context.Context, principalUUID, artifactID string) (*AgentArtifactContent, error) {
	s.principal, s.artifact, s.calls = principalUUID, artifactID, s.calls+1
	return &AgentArtifactContent{ArtifactID: artifactID, MediaType: "text/markdown", Content: "# Private digest"}, nil
}

func TestGatewayOwnsAuthenticatedAgentArtifactMetadata(t *testing.T) {
	t.Chdir("../../../..")
	t.Setenv("DIPOLE_CONFIG_FILE", "configs/config.dist.yaml")
	mr, _ := miniredis.Run()
	defer mr.Close()
	previousRedis := cache.RDB
	cache.RDB = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = cache.RDB.Close(); cache.RDB = previousRedis })
	core := httptest.NewServer(http.NotFoundHandler())
	defer core.Close()
	artifacts := &gatewayAgentArtifactStub{}
	server, err := newTestGatewayServer(core.URL, Dependencies{Messages: gatewayMessageStub{}, Core: gatewayCoreStub{}, AgentArtifacts: artifacts, Limiter: gatewayLimiterStub{}})
	if err != nil {
		t.Fatal(err)
	}
	unauthorized := httptest.NewRecorder()
	server.Engine().ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/v1/agent/artifacts/"+strings.Repeat("a", 64), nil))
	if unauthorized.Code != http.StatusUnauthorized || artifacts.calls != 0 {
		t.Fatalf("unauthorized code=%d calls=%d", unauthorized.Code, artifacts.calls)
	}
	token, _ := coreauth.NewTokenService().Issue(&model.User{UUID: "U100"})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/agent/artifacts/"+strings.Repeat("a", 64), nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	server.Engine().ServeHTTP(response, request)
	if response.Code != http.StatusOK || artifacts.principal != "U100" || !strings.Contains(response.Body.String(), `"artifactId"`) || strings.Contains(response.Body.String(), "private") {
		t.Fatalf("response=%d artifact=%+v body=%s", response.Code, artifacts, response.Body.String())
	}
}

func TestGatewayOwnsAuthenticatedAgentArtifactCatalog(t *testing.T) {
	t.Chdir("../../../..")
	t.Setenv("DIPOLE_CONFIG_FILE", "configs/config.dist.yaml")
	mr, _ := miniredis.Run()
	defer mr.Close()
	previousRedis := cache.RDB
	cache.RDB = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = cache.RDB.Close(); cache.RDB = previousRedis })
	core := httptest.NewServer(http.NotFoundHandler())
	defer core.Close()
	artifacts := &gatewayAgentArtifactStub{}
	server, err := newTestGatewayServer(core.URL, Dependencies{Messages: gatewayMessageStub{}, Core: gatewayCoreStub{}, AgentArtifacts: artifacts, Limiter: gatewayLimiterStub{}})
	if err != nil {
		t.Fatal(err)
	}
	unauthorized := httptest.NewRecorder()
	server.Engine().ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/v1/agent/artifacts", nil))
	if unauthorized.Code != http.StatusUnauthorized || artifacts.calls != 0 {
		t.Fatalf("unauthorized code=%d calls=%d", unauthorized.Code, artifacts.calls)
	}
	token, _ := coreauth.NewTokenService().Issue(&model.User{UUID: "U100"})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/agent/artifacts?limit=20", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	server.Engine().ServeHTTP(response, request)
	if response.Code != http.StatusOK || artifacts.principal != "U100" || !strings.Contains(response.Body.String(), `"artifacts"`) {
		t.Fatalf("response=%d artifact=%+v body=%s", response.Code, artifacts, response.Body.String())
	}
}

func TestGatewayOwnsAuthenticatedAgentArtifactDigestContent(t *testing.T) {
	t.Chdir("../../../..")
	t.Setenv("DIPOLE_CONFIG_FILE", "configs/config.dist.yaml")
	mr, _ := miniredis.Run()
	defer mr.Close()
	previousRedis := cache.RDB
	cache.RDB = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = cache.RDB.Close(); cache.RDB = previousRedis })
	core := httptest.NewServer(http.NotFoundHandler())
	defer core.Close()
	artifacts := &gatewayAgentArtifactStub{}
	server, err := newTestGatewayServer(core.URL, Dependencies{Messages: gatewayMessageStub{}, Core: gatewayCoreStub{}, AgentArtifacts: artifacts, Limiter: gatewayLimiterStub{}})
	if err != nil {
		t.Fatal(err)
	}
	artifactID := strings.Repeat("a", 64)
	unauthorized := httptest.NewRecorder()
	server.Engine().ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/v1/agent/artifacts/"+artifactID+"/content", nil))
	if unauthorized.Code != http.StatusUnauthorized || artifacts.calls != 0 {
		t.Fatalf("unauthorized code=%d calls=%d", unauthorized.Code, artifacts.calls)
	}
	token, _ := coreauth.NewTokenService().Issue(&model.User{UUID: "U100"})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/agent/artifacts/"+artifactID+"/content", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	server.Engine().ServeHTTP(response, request)
	if response.Code != http.StatusOK || artifacts.principal != "U100" || !strings.Contains(response.Body.String(), `"content":"# Private digest"`) || strings.Contains(response.Body.String(), "contentSha256") {
		t.Fatalf("response=%d artifact=%+v body=%s", response.Code, artifacts, response.Body.String())
	}
}
