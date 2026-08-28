package agentgrpc

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	grpcauth "github.com/JekYUlll/Dipole/internal/transport/grpc/auth"
	grpccommon "github.com/JekYUlll/Dipole/internal/transport/grpc/common"
	agentv1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/agent/v1"
	commonv1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/common/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	agentv1.UnimplementedAgentCapabilityServiceServer
	capability           application.AgentCapabilityV1
	resolver             application.AgentInvocationResolverV1
	admission            application.AgentRunAdmissionServiceV1
	approvals            application.AgentApprovalServiceV1
	approvalGrants       application.AgentApprovalGrantResolverV1
	controls             application.AgentTaskControlAuthorizerV1
	projections          application.AgentTaskWorkflowProjectionServiceV1
	repairs              application.AgentWorkflowRepairAuditServiceV1
	promotionControls    application.AgentRuntimePromotionControlServiceV1
	promotionEvidence    application.AgentRuntimePromotionEvidenceReviewServiceV1
	readinessPublisher   application.AgentMCPReadinessEvidencePublisherV1
	readinessResolver    application.AgentMCPReadinessEvidenceResolverV1
	artifacts            application.AgentArtifactServiceV1
	subscriptions        application.AgentEventSubscriptionResolverV1
	subscriptionControls application.AgentEventSubscriptionControlServiceV1
	definitionCatalog    application.AgentDefinitionCatalogServiceV1
	memories             application.AgentMemoryContextResolverV1
	memoryControls       application.AgentMemoryOwnerControlServiceV1
	toolAudits           application.AgentToolInvocationAuditServiceV1
	toolRounds           application.AgentMCPToolRoundServiceV1
	toolTerminals        application.AgentMCPToolInvocationTerminalServiceV1
	messageCommands      application.AgentMessageCommandExecutionV1
}

func (s *Server) WithMCPReadinessEvidencePublisher(publisher application.AgentMCPReadinessEvidencePublisherV1) (*Server, error) {
	if s == nil || publisher == nil {
		return nil, errors.New("Agent MCP readiness evidence Publisher is required")
	}
	s.readinessPublisher = publisher
	return s, nil
}

func (s *Server) WithMCPReadinessEvidenceResolver(resolver application.AgentMCPReadinessEvidenceResolverV1) (*Server, error) {
	if s == nil || resolver == nil {
		return nil, errors.New("Agent MCP readiness evidence Resolver is required")
	}
	s.readinessResolver = resolver
	return s, nil
}

func (s *Server) WithEventSubscriptionControls(controls application.AgentEventSubscriptionControlServiceV1) (*Server, error) {
	if s == nil || controls == nil {
		return nil, errors.New("Agent Event Subscription control service is required")
	}
	s.subscriptionControls = controls
	return s, nil
}

func (s *Server) WithDefinitionCatalog(catalog application.AgentDefinitionCatalogServiceV1) (*Server, error) {
	if s == nil || catalog == nil {
		return nil, errors.New("Agent Definition catalog service is required")
	}
	s.definitionCatalog = catalog
	return s, nil
}

func (s *Server) WithPromotionEvidence(evidence application.AgentRuntimePromotionEvidenceReviewServiceV1) (*Server, error) {
	if s == nil || evidence == nil {
		return nil, errors.New("Agent Runtime promotion evidence review service is required")
	}
	s.promotionEvidence = evidence
	return s, nil
}

func (s *Server) WithPromotionControls(controls application.AgentRuntimePromotionControlServiceV1) (*Server, error) {
	if s == nil || controls == nil {
		return nil, errors.New("Agent Runtime promotion control service is required")
	}
	s.promotionControls = controls
	return s, nil
}

func (s *Server) WithApprovalGrants(grants application.AgentApprovalGrantResolverV1) (*Server, error) {
	if s == nil || grants == nil {
		return nil, errors.New("Agent Approval grant resolver is required")
	}
	s.approvalGrants = grants
	return s, nil
}

func (s *Server) WithToolAudits(audits application.AgentToolInvocationAuditServiceV1) (*Server, error) {
	if s == nil || audits == nil {
		return nil, errors.New("Agent Tool invocation audit service is required")
	}
	s.toolAudits = audits
	return s, nil
}

func (s *Server) WithMCPToolRounds(rounds application.AgentMCPToolRoundServiceV1) (*Server, error) {
	if s == nil || rounds == nil {
		return nil, errors.New("Agent MCP Tool round service is required")
	}
	s.toolRounds = rounds
	return s, nil
}

func (s *Server) WithMCPToolTerminals(terminals application.AgentMCPToolInvocationTerminalServiceV1) (*Server, error) {
	if s == nil || terminals == nil {
		return nil, errors.New("Agent MCP Tool terminal service is required")
	}
	s.toolTerminals = terminals
	return s, nil
}

func (s *Server) WithMessageCommands(commands application.AgentMessageCommandExecutionV1) (*Server, error) {
	if s == nil || commands == nil {
		return nil, errors.New("Agent Message Command execution service is required")
	}
	s.messageCommands = commands
	return s, nil
}

func (s *Server) WithMemories(memories application.AgentMemoryContextResolverV1) (*Server, error) {
	if s == nil || memories == nil {
		return nil, errors.New("Agent Memory resolver is required")
	}
	s.memories = memories
	return s, nil
}

func (s *Server) WithMemoryOwnerControls(controls application.AgentMemoryOwnerControlServiceV1) (*Server, error) {
	if s == nil || controls == nil {
		return nil, errors.New("Agent Memory owner control service is required")
	}
	s.memoryControls = controls
	return s, nil
}

func (s *Server) ListOwnedMemories(ctx context.Context, request *agentv1.ListOwnedMemoriesRequest) (*agentv1.ListOwnedMemoriesResponse, error) {
	principal, err := agentMemoryOwnerV1(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	if s.memoryControls == nil {
		return nil, status.Error(codes.Unavailable, "Agent Memory owner control is unavailable")
	}
	var afterCreatedAt time.Time
	if request.GetAfterCreatedAtUnixMs() > 0 {
		afterCreatedAt = time.UnixMilli(request.GetAfterCreatedAtUnixMs()).UTC()
	}
	page, err := s.memoryControls.ListOwnedMemories(grpccommon.Correlation(ctx, request.GetContext()), application.AgentMemoryOwnerListRequestV1{
		TenantID: request.GetTenantId(), PrincipalUUID: principal, AfterCreatedAt: afterCreatedAt,
		AfterUUID: request.GetAfterMemoryId(), Limit: int(request.GetLimit()),
	})
	if err != nil {
		return nil, agentMemoryOwnerErrorV1(err)
	}
	response := &agentv1.ListOwnedMemoriesResponse{Memories: make([]*agentv1.AgentOwnedMemory, 0, len(page.Memories))}
	for _, item := range page.Memories {
		response.Memories = append(response.Memories, agentOwnedMemoryResponseV1(item))
	}
	if !page.NextCreatedAt.IsZero() {
		response.NextCreatedAtUnixMs = page.NextCreatedAt.UnixMilli()
		response.NextMemoryId = page.NextMemoryUUID
	}
	return response, nil
}

func (s *Server) RevokeOwnedMemory(ctx context.Context, request *agentv1.RevokeOwnedMemoryRequest) (*agentv1.AgentOwnedMemory, error) {
	principal, err := agentMemoryOwnerV1(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	if s.memoryControls == nil {
		return nil, status.Error(codes.Unavailable, "Agent Memory owner control is unavailable")
	}
	item, err := s.memoryControls.RevokeOwnedMemory(grpccommon.Correlation(ctx, request.GetContext()), application.AgentMemoryOwnerRevokeRequestV1{
		TenantID: request.GetTenantId(), PrincipalUUID: principal, MemoryUUID: request.GetMemoryId(), Reason: request.GetReason(),
	})
	if err != nil {
		return nil, agentMemoryOwnerErrorV1(err)
	}
	return agentOwnedMemoryResponseV1(*item), nil
}

func agentMemoryOwnerV1(ctx context.Context, requestContext *commonv1.RequestContext) (string, error) {
	authenticated, ok := grpcauth.CallerService(ctx)
	if !ok || authenticated != "dipole-gateway" || strings.TrimSpace(requestContext.GetCallerService()) != authenticated {
		return "", status.Error(codes.PermissionDenied, "only the authenticated Gateway may manage Agent Memories")
	}
	if _, err := grpccommon.Caller(ctx, requestContext); err != nil {
		return "", err
	}
	return grpccommon.Principal(requestContext)
}

func agentMemoryOwnerErrorV1(err error) error {
	switch {
	case errors.Is(err, application.ErrAgentMemoryDenied):
		return status.Error(codes.PermissionDenied, "Agent Memory access denied")
	case errors.Is(err, application.ErrAgentMemoryConflict):
		return status.Error(codes.Aborted, "Agent Memory changed concurrently")
	case errors.Is(err, application.ErrAgentMemoryInvalid):
		return status.Error(codes.FailedPrecondition, "Agent Memory request is invalid")
	default:
		return status.Error(codes.Internal, "Agent Memory owner control failed")
	}
}

func agentOwnedMemoryResponseV1(item application.AgentMemoryV1) *agentv1.AgentOwnedMemory {
	response := &agentv1.AgentOwnedMemory{
		MemoryId: item.MemoryUUID, AgentId: item.AgentUUID, MemoryType: string(item.MemoryType), Status: string(item.Status),
		ResourceType: item.ResourceType, ResourceId: item.ResourceID, Content: item.Content, CompactContent: item.CompactContent,
		Priority: item.Priority, Provenance: &agentv1.AgentMemoryProvenance{
			SourceType: item.Provenance.SourceType, SourceId: item.Provenance.SourceID, Sequence: item.Provenance.Sequence,
		}, ValidFromUnixMs: item.ValidFrom.UnixMilli(), CreatedAtUnixMs: item.CreatedAt.UnixMilli(),
		RevokedById: item.RevokedByUUID, RevokeReason: item.RevokeReason,
	}
	if item.ExpiresAt != nil {
		response.ExpiresAtUnixMs = item.ExpiresAt.UnixMilli()
	}
	if item.RevokedAt != nil {
		response.RevokedAtUnixMs = item.RevokedAt.UnixMilli()
	}
	return response
}

func (s *Server) ListContextMemories(ctx context.Context, request *agentv1.ListContextMemoriesRequest) (*agentv1.ListContextMemoriesResponse, error) {
	caller, err := authenticatedAgentArtifactCallerV1(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	if caller != "dipole-agent" || strings.TrimSpace(request.GetContext().GetPrincipalUserId()) != "" {
		return nil, status.Error(codes.PermissionDenied, "only the authenticated Agent runtime may list Context Memories")
	}
	if s.memories == nil {
		return nil, status.Error(codes.Unavailable, "Agent Memory resolver is unavailable")
	}
	items, err := s.memories.ResolveContextMemories(grpccommon.Correlation(ctx, request.GetContext()), request.GetTaskId(), request.GetRunId(), request.GetResourceType(), request.GetResourceId(), int(request.GetLimit()))
	if err != nil {
		switch {
		case errors.Is(err, application.ErrAgentMemoryDenied):
			return nil, status.Error(codes.PermissionDenied, "Agent Memory scope denied")
		case errors.Is(err, application.ErrAgentMemoryInvalid), errors.Is(err, application.ErrAgentExecutionPolicyDenied):
			return nil, status.Error(codes.FailedPrecondition, "Agent Memory request is invalid")
		default:
			return nil, status.Error(codes.Internal, "Agent Memory lookup failed")
		}
	}
	response := &agentv1.ListContextMemoriesResponse{Memories: make([]*agentv1.AgentContextMemory, 0, len(items))}
	for _, item := range items {
		response.Memories = append(response.Memories, &agentv1.AgentContextMemory{
			MemoryId: item.MemoryUUID, MemoryType: string(item.MemoryType), Content: item.Content,
			CompactContent: item.CompactContent, Priority: item.Priority,
			Provenance: &agentv1.AgentMemoryProvenance{SourceType: item.Provenance.SourceType, SourceId: item.Provenance.SourceID, Uri: item.Provenance.URI, Sequence: item.Provenance.Sequence},
		})
	}
	return response, nil
}

func (s *Server) WithEventSubscriptions(resolver application.AgentEventSubscriptionResolverV1) (*Server, error) {
	if s == nil || resolver == nil {
		return nil, errors.New("Agent Event Subscription resolver is required")
	}
	s.subscriptions = resolver
	return s, nil
}

func (s *Server) MatchEventSubscriptions(ctx context.Context, request *agentv1.MatchEventSubscriptionsRequest) (*agentv1.MatchEventSubscriptionsResponse, error) {
	caller, err := authenticatedAgentArtifactCallerV1(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	if caller != "dipole-agent" || strings.TrimSpace(request.GetContext().GetPrincipalUserId()) != "" {
		return nil, status.Error(codes.PermissionDenied, "only the authenticated Agent runtime may match Event Subscriptions")
	}
	if s.subscriptions == nil {
		return nil, status.Error(codes.Unavailable, "Agent Event Subscription resolver is unavailable")
	}
	items, err := s.subscriptions.MatchEventSubscriptions(grpccommon.Correlation(ctx, request.GetContext()), application.AgentEventSubscriptionMatchRequestV1{
		TenantID: request.GetTenantId(), AgentUUID: request.GetAgentId(), EventType: request.GetEventType(),
		ResourceType: request.GetResourceType(), ResourceID: request.GetResourceId(),
	})
	if err != nil {
		if errors.Is(err, application.ErrAgentSubscriptionInvalid) {
			return nil, status.Error(codes.FailedPrecondition, "Agent Event Subscription policy is invalid")
		}
		return nil, status.Error(codes.Internal, "Agent Event Subscription lookup failed")
	}
	response := &agentv1.MatchEventSubscriptionsResponse{Subscriptions: make([]*agentv1.AgentEventSubscription, 0, len(items))}
	for _, item := range items {
		response.Subscriptions = append(response.Subscriptions, agentEventSubscriptionResponseV1(item))
	}
	return response, nil
}

func (s *Server) CreateEventSubscription(ctx context.Context, request *agentv1.CreateEventSubscriptionRequest) (*agentv1.AgentEventSubscription, error) {
	principal, err := eventSubscriptionOwnerV1(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	if s.subscriptionControls == nil {
		return nil, status.Error(codes.Unavailable, "Agent Event Subscription control is unavailable")
	}
	item, err := s.subscriptionControls.Create(grpccommon.Correlation(ctx, request.GetContext()), principal, application.AgentEventSubscriptionCreateRequestV1{
		TenantID: request.GetTenantId(), DefinitionUUID: request.GetDefinitionId(), DefinitionVersion: request.GetDefinitionVersion(),
		EventType: request.GetEventType(), ResourceType: request.GetResourceType(), ResourceID: request.GetResourceId(),
		FilterKind: application.AgentSubscriptionFilterKindV1(request.GetFilterKind()), FilterJSON: request.GetFilterJson(),
	})
	if err != nil {
		return nil, eventSubscriptionControlErrorV1(err)
	}
	return agentEventSubscriptionResponseV1(*item), nil
}

func (s *Server) ListEligibleSubscriptionConversations(ctx context.Context, request *agentv1.ListEligibleSubscriptionConversationsRequest) (*agentv1.ListEligibleSubscriptionConversationsResponse, error) {
	principal, err := eventSubscriptionOwnerV1(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	if s.subscriptionControls == nil {
		return nil, status.Error(codes.Unavailable, "Agent Event Subscription control is unavailable")
	}
	items, err := s.subscriptionControls.ListEligibleConversations(grpccommon.Correlation(ctx, request.GetContext()), principal, application.AgentSubscriptionConversationOptionsRequestV1{
		TenantID: request.GetTenantId(), DefinitionUUID: request.GetDefinitionId(), DefinitionVersion: request.GetDefinitionVersion(),
	})
	if err != nil {
		return nil, eventSubscriptionControlErrorV1(err)
	}
	response := &agentv1.ListEligibleSubscriptionConversationsResponse{Conversations: make([]*agentv1.AgentSubscriptionConversationOption, 0, len(items))}
	for _, item := range items {
		response.Conversations = append(response.Conversations, &agentv1.AgentSubscriptionConversationOption{
			ConversationKey: item.ConversationKey, EventType: item.EventType,
		})
	}
	return response, nil
}

func (s *Server) ListEventSubscriptions(ctx context.Context, request *agentv1.ListEventSubscriptionsRequest) (*agentv1.ListEventSubscriptionsResponse, error) {
	principal, err := eventSubscriptionOwnerV1(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	if s.subscriptionControls == nil {
		return nil, status.Error(codes.Unavailable, "Agent Event Subscription control is unavailable")
	}
	page, err := s.subscriptionControls.List(grpccommon.Correlation(ctx, request.GetContext()), principal, application.AgentEventSubscriptionListRequestV1{
		TenantID: request.GetTenantId(), AfterUUID: request.GetAfterSubscriptionId(), Limit: int(request.GetLimit()),
	})
	if err != nil {
		return nil, eventSubscriptionControlErrorV1(err)
	}
	response := &agentv1.ListEventSubscriptionsResponse{Subscriptions: make([]*agentv1.AgentEventSubscription, 0, len(page.Subscriptions)), NextCursor: page.NextCursor}
	for _, item := range page.Subscriptions {
		response.Subscriptions = append(response.Subscriptions, agentEventSubscriptionResponseV1(item))
	}
	return response, nil
}

func (s *Server) RevokeEventSubscription(ctx context.Context, request *agentv1.RevokeEventSubscriptionRequest) (*agentv1.AgentEventSubscription, error) {
	principal, err := eventSubscriptionOwnerV1(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	if s.subscriptionControls == nil {
		return nil, status.Error(codes.Unavailable, "Agent Event Subscription control is unavailable")
	}
	item, err := s.subscriptionControls.Revoke(grpccommon.Correlation(ctx, request.GetContext()), principal, application.AgentEventSubscriptionRevokeRequestV1{
		TenantID: request.GetTenantId(), SubscriptionUUID: request.GetSubscriptionId(), Reason: request.GetReason(),
	})
	if err != nil {
		return nil, eventSubscriptionControlErrorV1(err)
	}
	return agentEventSubscriptionResponseV1(*item), nil
}

func (s *Server) ListAgentDefinitions(ctx context.Context, request *agentv1.ListAgentDefinitionsRequest) (*agentv1.ListAgentDefinitionsResponse, error) {
	principal, err := eventSubscriptionOwnerV1(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	if s.definitionCatalog == nil {
		return nil, status.Error(codes.Unavailable, "Agent Definition catalog is unavailable")
	}
	page, err := s.definitionCatalog.List(grpccommon.Correlation(ctx, request.GetContext()), principal, application.AgentDefinitionCatalogListRequestV1{
		TenantID: request.GetTenantId(), AfterDefinitionUUID: request.GetAfterDefinitionId(),
		AfterVersion: request.GetAfterVersion(), Limit: int(request.GetLimit()),
	})
	if err != nil {
		switch {
		case errors.Is(err, application.ErrAgentDefinitionCatalogInvalid):
			return nil, status.Error(codes.FailedPrecondition, "Agent Definition catalog request is invalid")
		case errors.Is(err, application.ErrAgentDefinitionCatalogConflict):
			return nil, status.Error(codes.Aborted, "Agent Definition catalog authority changed")
		default:
			return nil, status.Error(codes.Internal, "Agent Definition catalog lookup failed")
		}
	}
	response := &agentv1.ListAgentDefinitionsResponse{
		Definitions:      make([]*agentv1.AgentDefinitionCatalogItem, 0, len(page.Definitions)),
		NextDefinitionId: page.NextDefinitionUUID, NextVersion: page.NextVersion,
	}
	for _, item := range page.Definitions {
		definition := &agentv1.AgentDefinitionCatalogItem{
			DefinitionId: item.DefinitionUUID, Version: item.Version, AgentId: item.AgentUUID,
			ConversationScopes: append([]string(nil), item.ConversationScopes...),
			ValidFromUnixMs:    item.ValidFrom.UnixMilli(), CreatedAtUnixMs: item.CreatedAt.UnixMilli(), UpdatedAtUnixMs: item.UpdatedAt.UnixMilli(),
		}
		if item.ExpiresAt != nil {
			definition.ExpiresAtUnixMs = item.ExpiresAt.UnixMilli()
		}
		response.Definitions = append(response.Definitions, definition)
	}
	return response, nil
}

func eventSubscriptionOwnerV1(ctx context.Context, requestContext *commonv1.RequestContext) (string, error) {
	authenticated, ok := grpcauth.CallerService(ctx)
	if !ok || authenticated != "dipole-gateway" || strings.TrimSpace(requestContext.GetCallerService()) != authenticated {
		return "", status.Error(codes.PermissionDenied, "only the authenticated Gateway may manage Agent Event Subscriptions")
	}
	if _, err := grpccommon.Caller(ctx, requestContext); err != nil {
		return "", err
	}
	return grpccommon.Principal(requestContext)
}

func eventSubscriptionControlErrorV1(err error) error {
	switch {
	case errors.Is(err, application.ErrAgentSubscriptionDenied):
		return status.Error(codes.PermissionDenied, "Agent Event Subscription access denied")
	case errors.Is(err, application.ErrAgentSubscriptionConflict):
		return status.Error(codes.Aborted, "Agent Event Subscription changed concurrently")
	case errors.Is(err, application.ErrAgentSubscriptionInvalid):
		return status.Error(codes.FailedPrecondition, "Agent Event Subscription request is invalid")
	default:
		return status.Error(codes.Internal, "Agent Event Subscription control failed")
	}
}

func agentEventSubscriptionResponseV1(item application.AgentEventSubscriptionV1) *agentv1.AgentEventSubscription {
	response := &agentv1.AgentEventSubscription{
		SubscriptionId: item.SubscriptionUUID, DefinitionId: item.DefinitionUUID, DefinitionVersion: item.DefinitionVersion,
		TenantId: item.TenantID, AgentId: item.AgentUUID, EventType: item.EventType, ResourceType: item.ResourceType,
		ResourceId: item.ResourceID, FilterKind: string(item.FilterKind), FilterJson: item.FilterJSON, Status: string(item.Status),
		CreatedById: item.CreatedByUUID, RevokedById: item.RevokedByUUID, RevokeReason: item.RevokeReason,
	}
	if !item.CreatedAt.IsZero() {
		response.CreatedAtUnixMs = item.CreatedAt.UnixMilli()
	}
	if !item.UpdatedAt.IsZero() {
		response.UpdatedAtUnixMs = item.UpdatedAt.UnixMilli()
	}
	if item.RevokedAt != nil {
		response.RevokedAtUnixMs = item.RevokedAt.UnixMilli()
	}
	return response
}

func (s *Server) WithArtifacts(artifacts application.AgentArtifactServiceV1) (*Server, error) {
	if s == nil || artifacts == nil {
		return nil, errors.New("Agent Artifact service is required")
	}
	s.artifacts = artifacts
	return s, nil
}

func (s *Server) CreateArtifact(ctx context.Context, request *agentv1.CreateArtifactRequest) (*agentv1.CreateArtifactResponse, error) {
	caller, err := authenticatedAgentArtifactCallerV1(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	if caller != "dipole-agent" || strings.TrimSpace(request.GetContext().GetPrincipalUserId()) != "" {
		return nil, status.Error(codes.PermissionDenied, "only the authenticated Agent runtime may create Artifacts")
	}
	if s.artifacts == nil {
		return nil, status.Error(codes.Unavailable, "Agent Artifact storage is unavailable")
	}
	artifact, err := s.artifacts.Create(grpccommon.Correlation(ctx, request.GetContext()), application.AgentArtifactCreateV1{
		TenantID: request.GetTenantId(), TaskUUID: request.GetTaskId(), RunUUID: request.GetRunId(),
		ArtifactType: request.GetArtifactType(), Version: request.GetVersion(), Title: request.GetTitle(),
		MediaType: request.GetMediaType(), Content: request.GetContent(), Metadata: request.GetMetadataJson(),
	})
	if err != nil {
		return nil, mapAgentArtifactErrorV1(err)
	}
	return &agentv1.CreateArtifactResponse{Artifact: agentArtifactResponseV1(artifact)}, nil
}

func (s *Server) GetArtifact(ctx context.Context, request *agentv1.GetArtifactRequest) (*agentv1.GetArtifactResponse, error) {
	caller, err := authenticatedAgentArtifactCallerV1(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	if caller != "dipole-gateway" {
		return nil, status.Error(codes.PermissionDenied, "only the authenticated Gateway may retrieve Artifacts")
	}
	principal, err := grpccommon.Principal(request.GetContext())
	if err != nil {
		return nil, err
	}
	if s.artifacts == nil {
		return nil, status.Error(codes.Unavailable, "Agent Artifact storage is unavailable")
	}
	artifact, body, err := s.artifacts.GetForPrincipal(grpccommon.Correlation(ctx, request.GetContext()), principal, request.GetArtifactId())
	if err != nil {
		return nil, mapAgentArtifactErrorV1(err)
	}
	return &agentv1.GetArtifactResponse{Artifact: agentArtifactResponseV1(artifact), Content: body}, nil
}

func authenticatedAgentArtifactCallerV1(ctx context.Context, requestContext *commonv1.RequestContext) (string, error) {
	authenticated, ok := grpcauth.CallerService(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "authenticated service identity is required")
	}
	claimed, err := grpccommon.Caller(ctx, requestContext)
	if err != nil {
		return "", err
	}
	if authenticated != claimed {
		return "", status.Error(codes.PermissionDenied, "caller service does not match authenticated service")
	}
	return authenticated, nil
}

func agentArtifactResponseV1(value *application.AgentArtifactV1) *agentv1.AgentArtifact {
	if value == nil {
		return nil
	}
	return &agentv1.AgentArtifact{
		SchemaVersion: value.SchemaVersion, ArtifactId: value.ArtifactUUID, TaskId: value.TaskUUID, RunId: value.RunUUID,
		ArtifactType: value.ArtifactType, Version: value.Version, Title: value.Title, MediaType: value.MediaType,
		ContentSha256: value.ContentSHA256, SizeBytes: value.SizeBytes, MetadataJson: value.Metadata,
		CreatedAtUnixMs: value.CreatedAt.UnixMilli(),
	}
}

func mapAgentArtifactErrorV1(err error) error {
	if errors.Is(err, application.ErrAgentArtifactDenied) {
		return status.Error(codes.NotFound, "Agent Artifact unavailable")
	}
	if errors.Is(err, application.ErrAgentArtifactConflict) {
		return status.Error(codes.FailedPrecondition, "Agent Artifact evidence conflicts")
	}
	if errors.Is(err, application.ErrAgentArtifactInvalid) {
		return status.Error(codes.InvalidArgument, "Agent Artifact request is invalid")
	}
	return status.Error(codes.Internal, "Agent Artifact operation failed")
}

func NewServer(capability application.AgentCapabilityV1, resolver application.AgentInvocationResolverV1, admission application.AgentRunAdmissionServiceV1, approvals ...application.AgentApprovalServiceV1) (*Server, error) {
	if capability == nil || resolver == nil || admission == nil {
		return nil, errors.New("Agent Capability, Invocation resolver, and Run admission are required")
	}
	var approvalService application.AgentApprovalServiceV1
	if len(approvals) > 0 {
		approvalService = approvals[0]
	}
	return &Server{capability: capability, resolver: resolver, admission: admission, approvals: approvalService}, nil
}

func NewServerWithControl(capability application.AgentCapabilityV1, resolver application.AgentInvocationResolverV1, admission application.AgentRunAdmissionServiceV1, approvals application.AgentApprovalServiceV1, controls application.AgentTaskControlAuthorizerV1) (*Server, error) {
	server, err := NewServer(capability, resolver, admission, approvals)
	if err != nil {
		return nil, err
	}
	if controls == nil {
		return nil, errors.New("Agent Task control authorizer is required")
	}
	server.controls = controls
	return server, nil
}

func NewServerWithControlAndProjection(capability application.AgentCapabilityV1, resolver application.AgentInvocationResolverV1, admission application.AgentRunAdmissionServiceV1, approvals application.AgentApprovalServiceV1, controls application.AgentTaskControlAuthorizerV1, projections application.AgentTaskWorkflowProjectionServiceV1, repairs ...application.AgentWorkflowRepairAuditServiceV1) (*Server, error) {
	server, err := NewServerWithControl(capability, resolver, admission, approvals, controls)
	if err != nil {
		return nil, err
	}
	if projections == nil {
		return nil, errors.New("Agent Task Workflow projection service is required")
	}
	server.projections = projections
	if len(repairs) > 0 {
		if repairs[0] == nil {
			return nil, errors.New("Agent Workflow repair audit service is required")
		}
		server.repairs = repairs[0]
	}
	return server, nil
}

func (s *Server) ProposeWorkflowRepair(ctx context.Context, request *agentv1.ProposeWorkflowRepairRequest) (*agentv1.WorkflowRepairProposalResponse, error) {
	principal, err := workflowRepairOperatorV1(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	if s.repairs == nil {
		return nil, status.Error(codes.Unavailable, "Agent Workflow repair audit is unavailable")
	}
	proposal, err := s.repairs.Propose(ctx, principal, application.AgentWorkflowRepairProposalRequestV1{
		TaskUUID: request.GetTaskId(), Outcome: application.AgentWorkflowRepairOutcomeV1(request.GetOutcome()), TicketRef: request.GetTicketRef(), Reason: request.GetReason(),
		Projected: workflowRepairEvidenceFromRPCV1(request.GetProjected()), Temporal: workflowRepairEvidenceValueFromRPCV1(request.GetTemporal()),
		ProposedAt: time.UnixMilli(request.GetProposedAtUnixMs()), ExpiresAt: time.UnixMilli(request.GetExpiresAtUnixMs()),
	})
	if err != nil {
		return nil, workflowRepairErrorV1(err)
	}
	return workflowRepairProposalResponseV1(proposal), nil
}

func (s *Server) DecideWorkflowRepair(ctx context.Context, request *agentv1.DecideWorkflowRepairRequest) (*agentv1.WorkflowRepairProposalResponse, error) {
	principal, err := workflowRepairOperatorV1(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	if s.repairs == nil {
		return nil, status.Error(codes.Unavailable, "Agent Workflow repair audit is unavailable")
	}
	proposal, err := s.repairs.Decide(ctx, principal, request.GetProposalId(), application.AgentWorkflowRepairDecisionV1(request.GetDecision()))
	if err != nil {
		return nil, workflowRepairErrorV1(err)
	}
	return workflowRepairProposalResponseV1(proposal), nil
}

func (s *Server) GetWorkflowRepair(ctx context.Context, request *agentv1.GetWorkflowRepairRequest) (*agentv1.WorkflowRepairProposalResponse, error) {
	principal, err := workflowRepairOperatorV1(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	if s.repairs == nil {
		return nil, status.Error(codes.Unavailable, "Agent Workflow repair audit is unavailable")
	}
	proposal, err := s.repairs.Get(ctx, principal, request.GetProposalId())
	if err != nil {
		return nil, workflowRepairErrorV1(err)
	}
	return workflowRepairProposalResponseV1(proposal), nil
}

func (s *Server) ProposeRuntimePromotion(ctx context.Context, request *agentv1.ProposeRuntimePromotionRequest) (*agentv1.RuntimePromotionProposalResponse, error) {
	principal, err := runtimePromotionOperatorV1(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	if s.promotionControls == nil {
		return nil, status.Error(codes.Unavailable, "Agent Runtime promotion control is unavailable")
	}
	proposal, err := s.promotionControls.Propose(grpccommon.Correlation(ctx, request.GetContext()), principal, application.AgentRuntimePromotionProposalRequestV1{
		TenantID: request.GetTenantId(), RuntimeID: request.GetRuntimeId(), CandidateVersion: request.GetCandidateVersion(),
		DefinitionUUID: request.GetDefinitionId(), DefinitionVersion: request.GetDefinitionVersion(),
		EvidenceArtifactUUID: request.GetEvidenceArtifactId(), EvidenceSHA256: request.GetEvidenceSha256(), EvalSuiteSHA256: request.GetEvalSuiteSha256(),
		TicketRef: request.GetTicketRef(), Reason: request.GetReason(), ProposedAt: time.UnixMilli(request.GetProposedAtUnixMs()), ExpiresAt: time.UnixMilli(request.GetExpiresAtUnixMs()),
		GrantValidFrom: time.UnixMilli(request.GetGrantValidFromUnixMs()), GrantExpiresAt: time.UnixMilli(request.GetGrantExpiresAtUnixMs()),
	})
	if err != nil {
		return nil, runtimePromotionControlErrorV1(err)
	}
	return runtimePromotionProposalResponseV1(proposal), nil
}

func (s *Server) ReviewRuntimePromotion(ctx context.Context, request *agentv1.ReviewRuntimePromotionRequest) (*agentv1.RuntimePromotionProposalResponse, error) {
	principal, err := runtimePromotionOperatorV1(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	if s.promotionControls == nil {
		return nil, status.Error(codes.Unavailable, "Agent Runtime promotion control is unavailable")
	}
	proposal, err := s.promotionControls.Review(grpccommon.Correlation(ctx, request.GetContext()), principal, request.GetProposalId(), application.AgentRuntimePromotionReviewDecisionV1(request.GetDecision()))
	if err != nil {
		return nil, runtimePromotionControlErrorV1(err)
	}
	return runtimePromotionProposalResponseV1(proposal), nil
}

func (s *Server) GetRuntimePromotion(ctx context.Context, request *agentv1.GetRuntimePromotionRequest) (*agentv1.RuntimePromotionProposalResponse, error) {
	principal, err := runtimePromotionOperatorV1(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	if s.promotionControls == nil {
		return nil, status.Error(codes.Unavailable, "Agent Runtime promotion control is unavailable")
	}
	proposal, err := s.promotionControls.Get(grpccommon.Correlation(ctx, request.GetContext()), principal, request.GetTenantId(), request.GetProposalId())
	if err != nil {
		return nil, runtimePromotionControlErrorV1(err)
	}
	return runtimePromotionProposalResponseV1(proposal), nil
}

func (s *Server) GetRuntimePromotionEvidence(ctx context.Context, request *agentv1.GetRuntimePromotionEvidenceRequest) (*agentv1.RuntimePromotionEvidenceResponse, error) {
	principal, err := runtimePromotionOperatorV1(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	if s.promotionEvidence == nil {
		return nil, status.Error(codes.Unavailable, "Agent Runtime promotion evidence review is unavailable")
	}
	review, err := s.promotionEvidence.Get(grpccommon.Correlation(ctx, request.GetContext()), principal, request.GetTenantId(), request.GetProposalId())
	if err != nil {
		return nil, runtimePromotionControlErrorV1(err)
	}
	return &agentv1.RuntimePromotionEvidenceResponse{
		Proposal: runtimePromotionProposalResponseV1(review.Proposal), Artifact: agentArtifactResponseV1(review.Artifact), Content: review.Content,
	}, nil
}

func (s *Server) RevokeRuntimePromotion(ctx context.Context, request *agentv1.RevokeRuntimePromotionRequest) (*agentv1.RuntimePromotionGrantResponse, error) {
	principal, err := runtimePromotionOperatorV1(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	if s.promotionControls == nil {
		return nil, status.Error(codes.Unavailable, "Agent Runtime promotion control is unavailable")
	}
	grant, err := s.promotionControls.Revoke(grpccommon.Correlation(ctx, request.GetContext()), principal, request.GetGrantId(), request.GetTicketRef(), request.GetReason())
	if err != nil {
		return nil, runtimePromotionControlErrorV1(err)
	}
	response := &agentv1.RuntimePromotionGrantResponse{GrantId: grant.GrantUUID, TenantId: grant.TenantID, RuntimeId: grant.RuntimeID,
		CandidateVersion: grant.CandidateVersion, DefinitionId: grant.DefinitionUUID, DefinitionVersion: grant.DefinitionVersion,
		PolicyVersion: grant.PolicyVersion, EvidenceSha256: grant.EvidenceSHA256, EvalSuiteSha256: grant.EvalSuiteSHA256,
		GrantedById: grant.GrantedByUUID, ReviewedById: grant.ReviewedByUUID, ValidFromUnixMs: grant.ValidFrom.UnixMilli(), ExpiresAtUnixMs: grant.ExpiresAt.UnixMilli()}
	if grant.RevokedAt != nil {
		response.RevokedAtUnixMs = grant.RevokedAt.UnixMilli()
	}
	return response, nil
}

func (s *Server) PublishMcpReadinessEvidence(ctx context.Context, request *agentv1.PublishMcpReadinessEvidenceRequest) (*agentv1.PublishMcpReadinessEvidenceResponse, error) {
	caller, err := authenticatedAgentArtifactCallerV1(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	if caller != "dipole-agent" || strings.TrimSpace(request.GetContext().GetPrincipalUserId()) != "" {
		return nil, status.Error(codes.PermissionDenied, "only the authenticated Agent runtime may publish MCP readiness evidence")
	}
	if s.readinessPublisher == nil {
		return nil, status.Error(codes.Unavailable, "Agent MCP readiness evidence Publisher is unavailable")
	}
	if len(request.GetEvidenceJson()) == 0 || len(request.GetEvidenceJson()) > 16*1024 {
		return nil, status.Error(codes.InvalidArgument, "Agent MCP readiness evidence is invalid")
	}
	evidence, err := application.ParseAgentMCPReadinessEvidenceV1(request.GetEvidenceJson())
	if err != nil {
		return nil, readinessEvidenceResolveErrorV1(err)
	}
	requestContext := request.GetContext()
	record, created, err := s.readinessPublisher.PublishAgentMCPReadinessEvidence(
		grpccommon.Correlation(ctx, requestContext),
		caller,
		application.AgentMCPReadinessEvidenceRequestV1{
			TenantID: request.GetTenantId(), ProfileBindingSHA256: request.GetProfileBindingSha256(),
			RequestID: requestContext.GetRequestId(), TraceID: requestContext.GetTraceId(),
			ExpiresAt: time.UnixMilli(request.GetExpiresAtUnixMs()), Evidence: evidence,
		},
	)
	if err != nil {
		return nil, readinessEvidenceErrorV1(err)
	}
	if record == nil {
		return nil, status.Error(codes.Internal, "Agent MCP readiness evidence publication failed")
	}
	return &agentv1.PublishMcpReadinessEvidenceResponse{
		EvidenceId: record.EvidenceUUID, SchemaVersion: record.SchemaVersion,
		ProfileBindingSha256: record.ProfileBindingSHA256, RuntimeBindingSha256: record.RuntimeBindingSHA256,
		ContentSha256: record.ContentSHA256, Status: record.Status,
		CollectedAtUnixMs: record.CollectedAt.UnixMilli(), ExpiresAtUnixMs: record.ExpiresAt.UnixMilli(), Created: created,
	}, nil
}

func readinessEvidenceResolveErrorV1(err error) error {
	if errors.Is(err, application.ErrAgentMCPReadinessEvidenceInvalid) {
		return status.Error(codes.InvalidArgument, "Agent MCP readiness evidence lookup is invalid")
	}
	return status.Error(codes.Internal, "Agent MCP readiness evidence resolution failed")
}

func (s *Server) ResolveFreshMcpReadinessEvidence(ctx context.Context, request *agentv1.ResolveFreshMcpReadinessEvidenceRequest) (*agentv1.ResolveFreshMcpReadinessEvidenceResponse, error) {
	caller, err := authenticatedAgentArtifactCallerV1(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	if caller != "dipole-agent" || strings.TrimSpace(request.GetContext().GetPrincipalUserId()) != "" {
		return nil, status.Error(codes.PermissionDenied, "only the authenticated Agent runtime may resolve MCP readiness evidence")
	}
	if s.readinessResolver == nil {
		return nil, status.Error(codes.Unavailable, "Agent MCP readiness evidence Resolver is unavailable")
	}
	record, err := s.readinessResolver.ResolveFreshAgentMCPReadinessEvidence(
		grpccommon.Correlation(ctx, request.GetContext()), request.GetTenantId(), request.GetProfileBindingSha256(), request.GetRuntimeBindingSha256(),
	)
	if err != nil {
		return nil, readinessEvidenceResolveErrorV1(err)
	}
	if record == nil {
		return &agentv1.ResolveFreshMcpReadinessEvidenceResponse{Found: false}, nil
	}
	if record.Validate() != nil || record.TenantID != request.GetTenantId() ||
		record.ProfileBindingSHA256 != request.GetProfileBindingSha256() || record.RuntimeBindingSHA256 != request.GetRuntimeBindingSha256() {
		return nil, status.Error(codes.Internal, "Agent MCP readiness evidence resolution failed")
	}
	return &agentv1.ResolveFreshMcpReadinessEvidenceResponse{
		Found: true, EvidenceId: record.EvidenceUUID, SchemaVersion: record.SchemaVersion,
		ProfileBindingSha256: record.ProfileBindingSHA256, RuntimeBindingSha256: record.RuntimeBindingSHA256,
		ContentSha256: record.ContentSHA256, Status: record.Status,
		CollectedAtUnixMs: record.CollectedAt.UnixMilli(), ExpiresAtUnixMs: record.ExpiresAt.UnixMilli(),
	}, nil
}

func readinessEvidenceErrorV1(err error) error {
	switch {
	case errors.Is(err, application.ErrAgentMCPReadinessEvidenceInvalid):
		return status.Error(codes.InvalidArgument, "Agent MCP readiness evidence is invalid")
	case errors.Is(err, application.ErrAgentMCPReadinessEvidenceConflict):
		return status.Error(codes.FailedPrecondition, "Agent MCP readiness evidence conflicts with immutable history")
	default:
		return status.Error(codes.Internal, "Agent MCP readiness evidence publication failed")
	}
}

func runtimePromotionOperatorV1(ctx context.Context, requestContext *commonv1.RequestContext) (string, error) {
	authenticated, ok := grpcauth.CallerService(ctx)
	if !ok || authenticated != "dipole-gateway" || strings.TrimSpace(requestContext.GetCallerService()) != authenticated {
		return "", status.Error(codes.PermissionDenied, "only the authenticated Gateway may submit Runtime promotion control requests")
	}
	if _, err := grpccommon.Caller(ctx, requestContext); err != nil {
		return "", err
	}
	return grpccommon.Principal(requestContext)
}

func runtimePromotionProposalResponseV1(value *application.AgentRuntimePromotionProposalV1) *agentv1.RuntimePromotionProposalResponse {
	if value == nil {
		return nil
	}
	response := &agentv1.RuntimePromotionProposalResponse{ProposalId: value.ProposalUUID, TenantId: value.TenantID, RuntimeId: value.RuntimeID,
		CandidateVersion: value.CandidateVersion, DefinitionId: value.DefinitionUUID, DefinitionVersion: value.DefinitionVersion,
		EvidenceArtifactId: value.EvidenceArtifactUUID, EvidenceSha256: value.EvidenceSHA256, EvalSuiteSha256: value.EvalSuiteSHA256,
		ProposerId: value.ProposerUUID, TicketRef: value.TicketRef, Reason: value.Reason, Status: string(value.Status), GrantId: value.GrantUUID,
		ProposedAtUnixMs: value.ProposedAt.UnixMilli(), ExpiresAtUnixMs: value.ExpiresAt.UnixMilli(),
		GrantValidFromUnixMs: value.GrantValidFrom.UnixMilli(), GrantExpiresAtUnixMs: value.GrantExpiresAt.UnixMilli()}
	if value.DecidedAt != nil {
		response.DecidedAtUnixMs = value.DecidedAt.UnixMilli()
	}
	return response
}

func runtimePromotionControlErrorV1(err error) error {
	switch {
	case errors.Is(err, application.ErrAgentRuntimePromotionControlDenied):
		return status.Error(codes.PermissionDenied, "Runtime promotion control request denied")
	case errors.Is(err, application.ErrAgentRuntimePromotionControlConflict):
		return status.Error(codes.FailedPrecondition, "Runtime promotion control evidence conflicts")
	default:
		return status.Error(codes.Internal, "Runtime promotion control operation failed")
	}
}

func workflowRepairOperatorV1(ctx context.Context, requestContext *commonv1.RequestContext) (string, error) {
	authenticated, ok := grpcauth.CallerService(ctx)
	if !ok || authenticated != "dipole-gateway" || strings.TrimSpace(requestContext.GetCallerService()) != authenticated {
		return "", status.Error(codes.PermissionDenied, "only the authenticated Gateway may submit Workflow repair audit requests")
	}
	// Caller still verifies the mTLS/shared-secret identity against the claimed service.
	if _, err := grpccommon.Caller(ctx, requestContext); err != nil {
		return "", err
	}
	return grpccommon.Principal(requestContext)
}

func workflowRepairEvidenceFromRPCV1(value *agentv1.WorkflowRepairEvidence) *application.AgentWorkflowEvidenceV1 {
	if value == nil {
		return nil
	}
	result := workflowRepairEvidenceValueFromRPCV1(value)
	return &result
}
func workflowRepairEvidenceValueFromRPCV1(value *agentv1.WorkflowRepairEvidence) application.AgentWorkflowEvidenceV1 {
	if value == nil {
		return application.AgentWorkflowEvidenceV1{}
	}
	return application.AgentWorkflowEvidenceV1{WorkflowID: value.GetWorkflowId(), WorkflowRunID: value.GetWorkflowRunId(), Status: value.GetStatus(), Revision: value.GetRevision()}
}
func workflowRepairEvidenceToRPCV1(value *application.AgentWorkflowEvidenceV1) *agentv1.WorkflowRepairEvidence {
	if value == nil {
		return nil
	}
	return &agentv1.WorkflowRepairEvidence{WorkflowId: value.WorkflowID, WorkflowRunId: value.WorkflowRunID, Status: value.Status, Revision: value.Revision}
}
func workflowRepairProposalResponseV1(value *application.AgentWorkflowRepairProposalV1) *agentv1.WorkflowRepairProposalResponse {
	if value == nil {
		return nil
	}
	temporal := value.Temporal
	return &agentv1.WorkflowRepairProposalResponse{ProposalId: value.ProposalUUID, TaskId: value.TaskUUID, Outcome: string(value.Outcome), Action: value.Action,
		ProposerId: value.ProposerUUID, TicketRef: value.TicketRef, Reason: value.Reason, Projected: workflowRepairEvidenceToRPCV1(value.Projected), Temporal: workflowRepairEvidenceToRPCV1(&temporal),
		EvidenceSha256: value.EvidenceSHA256, Status: string(value.Status), RequiredApprovals: uint32(value.RequiredApprovals), ProposedAtUnixMs: value.ProposedAt.UnixMilli(), ExpiresAtUnixMs: value.ExpiresAt.UnixMilli()}
}
func workflowRepairErrorV1(err error) error {
	if errors.Is(err, application.ErrAgentWorkflowRepairDenied) {
		return status.Error(codes.PermissionDenied, "Agent Workflow repair access denied")
	}
	if errors.Is(err, application.ErrAgentWorkflowRepairConflict) {
		return status.Error(codes.FailedPrecondition, "Agent Workflow repair evidence conflicts")
	}
	return status.Error(codes.Internal, "Agent Workflow repair audit failed")
}

func (s *Server) AuthorizeTaskControl(ctx context.Context, request *agentv1.AuthorizeTaskControlRequest) (*agentv1.AuthorizeTaskControlResponse, error) {
	if _, err := grpccommon.Caller(ctx, request.GetContext()); err != nil {
		return nil, err
	}
	if s.controls == nil || strings.TrimSpace(request.GetContext().GetPrincipalUserId()) != "" {
		return nil, status.Error(codes.InvalidArgument, "Agent Task control authorization is invalid")
	}
	authorization, err := s.controls.AuthorizeTaskControl(ctx, request.GetTaskId(), request.GetPrincipalUserId())
	if err != nil {
		if errors.Is(err, application.ErrAgentExecutionPolicyDenied) {
			return nil, status.Error(codes.NotFound, "Agent Task unavailable")
		}
		return nil, status.Error(codes.Internal, "Agent Task control authorization failed")
	}
	response := &agentv1.AuthorizeTaskControlResponse{TaskId: authorization.TaskUUID, TaskStatus: string(authorization.Status)}
	if authorization.Workflow != nil {
		response.WorkflowId = authorization.Workflow.WorkflowID
		response.WorkflowRunId = authorization.Workflow.RunID
		response.WorkflowStatus = string(authorization.Workflow.Status)
		response.WorkflowRevision = authorization.Workflow.Revision
	}
	return response, nil
}

func (s *Server) ResolveMcpContext(ctx context.Context, request *agentv1.ResolveMcpContextRequest) (*agentv1.ResolveMcpContextResponse, error) {
	caller, err := grpccommon.Caller(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	if caller != "dipole-agent" || strings.TrimSpace(request.GetContext().GetPrincipalUserId()) != "" {
		return nil, status.Error(codes.PermissionDenied, "only the authenticated Agent runtime may resolve MCP context")
	}
	invocation, err := s.resolver.Resolve(grpccommon.Correlation(ctx, request.GetContext()), request.GetTaskId(), request.GetRunId())
	if err != nil {
		if errors.Is(err, application.ErrAgentExecutionPolicyDenied) {
			return nil, status.Error(codes.NotFound, "Agent MCP context unavailable")
		}
		return nil, status.Error(codes.Internal, "Agent MCP context lookup failed")
	}
	if strings.TrimSpace(request.GetPrincipalUserId()) == "" || request.GetPrincipalUserId() != invocation.PrincipalUUID {
		return nil, status.Error(codes.NotFound, "Agent MCP context unavailable")
	}
	if invocation.RuntimeID != "dipole-agent" || (invocation.Mode != "shadow" && invocation.Mode != "active") {
		return nil, status.Error(codes.NotFound, "Agent MCP context unavailable")
	}
	if err := application.ValidateAgentApprovedCapabilitiesV1(invocation.Mode, invocation.ApprovedCapabilities); err != nil {
		return nil, status.Error(codes.NotFound, "Agent MCP context unavailable")
	}
	response := &agentv1.ResolveMcpContextResponse{
		TenantId: invocation.TenantID, PrincipalUserId: invocation.PrincipalUUID, AgentId: invocation.AgentUUID,
		DelegatedByUserId: invocation.DelegatedByUUID, Permissions: append([]string(nil), invocation.Permissions...),
		RuntimeId: invocation.RuntimeID, Mode: invocation.Mode,
		ApprovedCapabilities: append([]string(nil), invocation.ApprovedCapabilities...),
		ResourceScopes:       make([]*agentv1.AgentResourceScope, 0, len(invocation.ResourceScopes)),
	}
	for _, scope := range invocation.ResourceScopes {
		response.ResourceScopes = append(response.ResourceScopes, &agentv1.AgentResourceScope{
			ResourceType: scope.ResourceType, ResourceId: scope.ResourceID, Actions: append([]string(nil), scope.Actions...),
		})
	}
	return response, nil
}

func (s *Server) BeginMcpToolInvocation(ctx context.Context, request *agentv1.BeginMcpToolInvocationRequest) (*agentv1.BeginMcpToolInvocationResponse, error) {
	if err := s.authorizeMcpToolAuditCallerV1(ctx, request.GetContext()); err != nil {
		return nil, err
	}
	if s.toolAudits == nil {
		return nil, status.Error(codes.Unavailable, "Agent Tool invocation audit is unavailable")
	}
	record, err := s.toolAudits.Begin(grpccommon.Correlation(ctx, request.GetContext()), application.AgentToolInvocationBeginV1{
		InvocationUUID: request.GetInvocationId(), TaskUUID: request.GetTaskId(), RunUUID: request.GetRunId(),
		Transport: application.AgentToolTransportMCP, ToolName: request.GetToolName(), CapabilityID: request.GetCapabilityId(),
		ArgumentsSHA256: request.GetArgumentsSha256(), ProfileID: request.GetProfileId(), ServerID: request.GetServerId(), ArgumentsJSON: string(request.GetArgumentsJson()),
		RequestID: request.GetContext().GetRequestId(), TraceID: request.GetContext().GetTraceId(), ApprovalUUID: request.GetApprovalId(),
	})
	if err != nil {
		return nil, mapAgentToolInvocationErrorV1(err)
	}
	return &agentv1.BeginMcpToolInvocationResponse{InvocationId: record.InvocationUUID, Status: string(record.Status)}, nil
}

func (s *Server) ResolveMcpToolCommand(ctx context.Context, request *agentv1.ResolveMcpToolCommandRequest) (*agentv1.ResolveMcpToolCommandResponse, error) {
	if err := s.authorizeMcpToolAuditCallerV1(ctx, request.GetContext()); err != nil {
		return nil, err
	}
	if s.toolAudits == nil {
		return nil, status.Error(codes.Unavailable, "Agent Tool command resolver is unavailable")
	}
	command, err := s.toolAudits.ResolveCommand(grpccommon.Correlation(ctx, request.GetContext()), request.GetTaskId(), request.GetRunId(), request.GetInvocationId())
	if err != nil {
		return nil, mapAgentToolInvocationErrorV1(err)
	}
	return &agentv1.ResolveMcpToolCommandResponse{
		InvocationId: command.InvocationUUID, TenantId: command.TenantID, PrincipalUserId: command.PrincipalUUID, AgentId: command.AgentUUID,
		TaskId: command.TaskUUID, RunId: command.RunUUID, ProfileId: command.ProfileID, ServerId: command.ServerID,
		ToolName: command.ToolName, CapabilityId: command.CapabilityID, ArgumentsJson: []byte(command.ArgumentsJSON), ArgumentsSha256: command.ArgumentsSHA256,
		StartedAtUnixMs: command.StartedAt.UnixMilli(), Status: string(command.Status),
	}, nil
}

func (s *Server) ClaimMcpToolRound(ctx context.Context, request *agentv1.ClaimMcpToolRoundRequest) (*agentv1.ClaimMcpToolRoundResponse, error) {
	if err := s.authorizeMcpToolAuditCallerV1(ctx, request.GetContext()); err != nil {
		return nil, err
	}
	if s.toolRounds == nil {
		return nil, status.Error(codes.Unavailable, "Agent MCP Tool round receipt is unavailable")
	}
	if request.GetRoundNumber() > 1 {
		return nil, mapAgentMCPToolRoundErrorV1(application.ErrAgentMCPToolRoundInvalid)
	}
	result, err := s.toolRounds.Claim(grpccommon.Correlation(ctx, request.GetContext()), application.AgentMCPToolRoundClaimV1{
		RoundUUID: request.GetRoundId(), InvocationUUID: request.GetInvocationId(), TaskUUID: request.GetTaskId(), RunUUID: request.GetRunId(),
		RoundNumber: uint8(request.GetRoundNumber()), RequestSHA256: request.GetRequestSha256(), OwnerTokenSHA256: request.GetOwnerTokenSha256(),
	})
	if err != nil {
		return nil, mapAgentMCPToolRoundErrorV1(err)
	}
	return &agentv1.ClaimMcpToolRoundResponse{
		RoundId: request.GetRoundId(), Outcome: string(result.Outcome), ResultJson: []byte(result.ResultJSON),
		ResultSha256: result.ResultSHA256, ErrorCode: result.ErrorCode,
	}, nil
}

func (s *Server) FinishMcpToolRound(ctx context.Context, request *agentv1.FinishMcpToolRoundRequest) (*agentv1.FinishMcpToolRoundResponse, error) {
	if err := s.authorizeMcpToolAuditCallerV1(ctx, request.GetContext()); err != nil {
		return nil, err
	}
	if s.toolRounds == nil {
		return nil, status.Error(codes.Unavailable, "Agent MCP Tool round receipt is unavailable")
	}
	finish := application.AgentMCPToolRoundFinishV1{
		RoundUUID: request.GetRoundId(), OwnerTokenSHA256: request.GetOwnerTokenSha256(),
		Status: application.AgentMCPToolRoundStatusV1(request.GetStatus()), ResultJSON: string(request.GetResultJson()),
		ResultSHA256: request.GetResultSha256(), ErrorCode: request.GetErrorCode(),
	}
	if err := s.toolRounds.Finish(grpccommon.Correlation(ctx, request.GetContext()), finish); err != nil {
		return nil, mapAgentMCPToolRoundErrorV1(err)
	}
	return &agentv1.FinishMcpToolRoundResponse{RoundId: finish.RoundUUID, Status: string(finish.Status)}, nil
}

func (s *Server) FinishMcpToolInvocation(ctx context.Context, request *agentv1.FinishMcpToolInvocationRequest) (*agentv1.FinishMcpToolInvocationResponse, error) {
	if err := s.authorizeMcpToolAuditCallerV1(ctx, request.GetContext()); err != nil {
		return nil, err
	}
	if s.toolAudits == nil {
		return nil, status.Error(codes.Unavailable, "Agent Tool invocation audit is unavailable")
	}
	command, err := s.toolAudits.ResolveCommand(grpccommon.Correlation(ctx, request.GetContext()), request.GetTaskId(), request.GetRunId(), request.GetInvocationId())
	if err != nil {
		return nil, mapAgentToolInvocationErrorV1(err)
	}
	if command.ProfileID != "" {
		return nil, status.Error(codes.PermissionDenied, "external MCP Tool invocation must finish from its durable round")
	}
	finish := application.AgentToolInvocationFinishV1{
		InvocationUUID: request.GetInvocationId(), TaskUUID: request.GetTaskId(), RunUUID: request.GetRunId(),
		Status: application.AgentToolInvocationStatusV1(request.GetStatus()), ResultSHA256: request.GetResultSha256(),
		ResultBytes: request.GetResultBytes(), LatencyMS: request.GetLatencyMs(), ErrorCode: request.GetErrorCode(),
	}
	if reference := request.GetActionReference(); reference != nil {
		finish.ActionReference = &application.AgentToolActionReferenceV1{
			ResourceType: application.AgentToolActionResourceTypeV1(reference.GetResourceType()), ResourceUUID: reference.GetResourceId(),
			CommandKind: application.AgentMessageCommandKindV1(reference.GetCommandKind()), CommandID: reference.GetCommandId(),
		}
	}
	if err := s.toolAudits.Finish(grpccommon.Correlation(ctx, request.GetContext()), finish); err != nil {
		return nil, mapAgentToolInvocationErrorV1(err)
	}
	return &agentv1.FinishMcpToolInvocationResponse{InvocationId: finish.InvocationUUID, Status: string(finish.Status)}, nil
}

func (s *Server) FinishMcpToolInvocationFromRound(ctx context.Context, request *agentv1.FinishMcpToolInvocationFromRoundRequest) (*agentv1.FinishMcpToolInvocationFromRoundResponse, error) {
	if err := s.authorizeMcpToolAuditCallerV1(ctx, request.GetContext()); err != nil {
		return nil, err
	}
	if s.toolTerminals == nil {
		return nil, status.Error(codes.Unavailable, "Agent MCP Tool terminal service is unavailable")
	}
	invocation, err := s.toolTerminals.FinishFromRound(grpccommon.Correlation(ctx, request.GetContext()), application.AgentMCPToolInvocationTerminalRequestV1{
		TaskUUID: request.GetTaskId(), RunUUID: request.GetRunId(), InvocationUUID: request.GetInvocationId(), RoundUUID: request.GetRoundId(),
	})
	if err != nil {
		return nil, mapAgentMCPToolTerminalErrorV1(err)
	}
	return &agentv1.FinishMcpToolInvocationFromRoundResponse{InvocationId: invocation.InvocationUUID, Status: string(invocation.Status)}, nil
}

func (s *Server) ExecuteMcpMessageCommand(ctx context.Context, request *agentv1.ExecuteMcpMessageCommandRequest) (*agentv1.ExecuteMcpMessageCommandResponse, error) {
	if err := s.authorizeMcpToolAuditCallerV1(ctx, request.GetContext()); err != nil {
		return nil, err
	}
	if s.messageCommands == nil {
		return nil, status.Error(codes.Unavailable, "Agent Message Command execution is unavailable")
	}
	result, err := s.messageCommands.Execute(grpccommon.Correlation(ctx, request.GetContext()), application.AgentMessageCommandExecutionRequestV1{
		TaskUUID: request.GetTaskId(), RunUUID: request.GetRunId(), InvocationUUID: request.GetInvocationId(),
		Kind: application.AgentMessageCommandKindV1(request.GetCommandKind()), Content: request.GetContent(),
	})
	if err != nil {
		switch {
		case errors.Is(err, application.ErrAgentCommandDenied):
			return nil, status.Error(codes.PermissionDenied, "Agent Message Command denied")
		case errors.Is(err, application.ErrAgentCommandConflict):
			return nil, status.Error(codes.Aborted, "Agent Message Command result conflicts")
		default:
			return nil, status.Error(codes.Internal, "Agent Message Command execution failed")
		}
	}
	return &agentv1.ExecuteMcpMessageCommandResponse{
		ActionReference: &agentv1.AgentToolActionReference{
			ResourceType: string(application.AgentToolActionResourceMessage), ResourceId: result.MessageUUID,
			CommandKind: string(result.Kind), CommandId: result.CommandID,
		},
		ClientMessageId: result.ClientMessageID,
	}, nil
}

func (s *Server) authorizeMcpToolAuditCallerV1(ctx context.Context, requestContext *commonv1.RequestContext) error {
	caller, err := grpccommon.Caller(ctx, requestContext)
	if err != nil {
		return err
	}
	if caller != "dipole-agent" || strings.TrimSpace(requestContext.GetPrincipalUserId()) != "" {
		return status.Error(codes.PermissionDenied, "only the authenticated Agent runtime may audit MCP Tool invocations")
	}
	return nil
}

func mapAgentToolInvocationErrorV1(err error) error {
	switch {
	case errors.Is(err, application.ErrAgentToolInvocationInvalid):
		return status.Error(codes.InvalidArgument, "Agent Tool invocation evidence is invalid")
	case errors.Is(err, application.ErrAgentToolInvocationDenied):
		return status.Error(codes.PermissionDenied, "Agent Tool invocation denied")
	case errors.Is(err, application.ErrAgentToolInvocationConflict):
		return status.Error(codes.Aborted, "Agent Tool invocation state conflicts")
	default:
		return status.Error(codes.Internal, "Agent Tool invocation audit failed")
	}
}

func mapAgentMCPToolRoundErrorV1(err error) error {
	switch {
	case errors.Is(err, application.ErrAgentMCPToolRoundInvalid):
		return status.Error(codes.InvalidArgument, "Agent MCP Tool round evidence is invalid")
	case errors.Is(err, application.ErrAgentMCPToolRoundDenied):
		return status.Error(codes.PermissionDenied, "Agent MCP Tool round denied")
	case errors.Is(err, application.ErrAgentMCPToolRoundConflict):
		return status.Error(codes.Aborted, "Agent MCP Tool round state conflicts")
	default:
		return status.Error(codes.Internal, "Agent MCP Tool round receipt failed")
	}
}

func mapAgentMCPToolTerminalErrorV1(err error) error {
	if errors.Is(err, application.ErrAgentToolInvocationInvalid) || errors.Is(err, application.ErrAgentToolInvocationDenied) || errors.Is(err, application.ErrAgentToolInvocationConflict) {
		return mapAgentToolInvocationErrorV1(err)
	}
	return mapAgentMCPToolRoundErrorV1(err)
}

func (s *Server) ProjectTaskWorkflowState(ctx context.Context, request *agentv1.ProjectTaskWorkflowStateRequest) (*agentv1.ProjectTaskWorkflowStateResponse, error) {
	if _, err := grpccommon.Caller(ctx, request.GetContext()); err != nil {
		return nil, err
	}
	if s.projections == nil || strings.TrimSpace(request.GetContext().GetPrincipalUserId()) != "" {
		return nil, status.Error(codes.InvalidArgument, "Agent Task Workflow projection is invalid")
	}
	projection, err := s.projections.Project(ctx, application.AgentTaskWorkflowProjectionRequestV1{
		Projection: application.AgentTaskWorkflowProjectionV1{
			TaskUUID: request.GetTaskId(), WorkflowID: request.GetWorkflowId(), RunID: request.GetWorkflowRunId(),
			Status: application.AgentTaskWorkflowStatusV1(request.GetWorkflowStatus()), Revision: request.GetWorkflowRevision(),
		},
		RunUUID: request.GetRunId(), RuntimeID: "dipole-agent", Mode: "shadow",
	})
	if err != nil {
		if errors.Is(err, application.ErrAgentExecutionPolicyDenied) {
			return nil, status.Error(codes.PermissionDenied, "Agent Task Workflow projection denied")
		}
		if errors.Is(err, application.ErrAgentWorkflowProjectionConflict) {
			return nil, status.Error(codes.FailedPrecondition, "Agent Task Workflow projection conflicts")
		}
		return nil, status.Error(codes.Internal, "Agent Task Workflow projection failed")
	}
	return &agentv1.ProjectTaskWorkflowStateResponse{
		TaskId: projection.TaskUUID, WorkflowId: projection.WorkflowID, WorkflowRunId: projection.RunID,
		WorkflowStatus: string(projection.Status), WorkflowRevision: projection.Revision,
	}, nil
}

func (s *Server) ListTaskWorkflowProjectionSnapshots(ctx context.Context, request *agentv1.ListTaskWorkflowProjectionSnapshotsRequest) (*agentv1.ListTaskWorkflowProjectionSnapshotsResponse, error) {
	if _, err := grpccommon.Caller(ctx, request.GetContext()); err != nil {
		return nil, err
	}
	if s.projections == nil || strings.TrimSpace(request.GetContext().GetPrincipalUserId()) != "" {
		return nil, status.Error(codes.InvalidArgument, "Agent Task Workflow projection page is invalid")
	}
	page, err := s.projections.ListProjectionSnapshots(ctx, request.GetAfterTaskId(), int(request.GetPageSize()))
	if err != nil {
		if errors.Is(err, application.ErrAgentExecutionPolicyDenied) {
			return nil, status.Error(codes.InvalidArgument, "Agent Task Workflow projection page is invalid")
		}
		return nil, status.Error(codes.Internal, "Agent Task Workflow projection page failed")
	}
	response := &agentv1.ListTaskWorkflowProjectionSnapshotsResponse{NextCursor: page.NextCursor}
	for _, task := range page.Tasks {
		snapshot := &agentv1.TaskWorkflowProjectionSnapshot{TaskId: task.TaskUUID}
		if task.Workflow != nil {
			snapshot.HasWorkflow = true
			snapshot.WorkflowId = task.Workflow.WorkflowID
			snapshot.WorkflowRunId = task.Workflow.RunID
			snapshot.WorkflowStatus = string(task.Workflow.Status)
			snapshot.WorkflowRevision = task.Workflow.Revision
		}
		response.Tasks = append(response.Tasks, snapshot)
	}
	return response, nil
}

func (s *Server) RequestApproval(ctx context.Context, request *agentv1.RequestApprovalRequest) (*agentv1.ApprovalResponse, error) {
	if _, err := grpccommon.Caller(ctx, request.GetContext()); err != nil {
		return nil, err
	}
	if s.approvals == nil || strings.TrimSpace(request.GetContext().GetPrincipalUserId()) != "" || request.GetResourceScope() == nil {
		return nil, status.Error(codes.InvalidArgument, "Agent Approval request is invalid")
	}
	approval, err := s.approvals.Request(ctx, application.AgentApprovalRequestV1{
		TaskUUID: request.GetTaskId(), RunUUID: request.GetRunId(), RuntimeID: "dipole-agent", Mode: "shadow",
		Approval: application.AgentApprovalV1{
			ApprovalUUID: request.GetApprovalId(), TaskUUID: request.GetTaskId(), CapabilityID: request.GetCapabilityId(),
			ResourceScope: application.AgentResourceScopeV1{ResourceType: request.GetResourceScope().GetResourceType(), ResourceID: request.GetResourceScope().GetResourceId(), Actions: request.GetResourceScope().GetActions()},
			ScopeSHA256:   request.GetScopeSha256(), ArgumentsSHA256: request.GetArgumentsSha256(), NonceSHA256: request.GetNonceSha256(),
			Status: application.AgentApprovalStatusPending, ExpiresAt: time.UnixMilli(request.GetExpiresAtUnixMs()).UTC(),
		},
	})
	if err != nil {
		return nil, mapApprovalError(err)
	}
	return approvalResponse(approval), nil
}

func (s *Server) ResolveApproval(ctx context.Context, request *agentv1.ResolveApprovalRequest) (*agentv1.ApprovalResponse, error) {
	if _, err := grpccommon.Caller(ctx, request.GetContext()); err != nil {
		return nil, err
	}
	if s.approvals == nil || strings.TrimSpace(request.GetContext().GetPrincipalUserId()) != "" {
		return nil, status.Error(codes.InvalidArgument, "Agent Approval resolution is invalid")
	}
	approval, err := s.approvals.Resolve(ctx, application.AgentApprovalResolutionV1{
		TaskUUID: request.GetTaskId(), RunUUID: request.GetRunId(), RuntimeID: "dipole-agent", Mode: "shadow",
		ApprovalUUID: request.GetApprovalId(), ActorUUID: request.GetActorUserId(), Decision: application.AgentApprovalDecisionV1(request.GetDecision()),
	})
	if err != nil {
		return nil, mapApprovalError(err)
	}
	return approvalResponse(approval), nil
}

func (s *Server) ConsumeApproval(ctx context.Context, request *agentv1.ConsumeApprovalRequest) (*agentv1.ConsumeApprovalResponse, error) {
	caller, err := grpccommon.Caller(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	if caller != "dipole-agent" {
		return nil, status.Error(codes.PermissionDenied, "Agent Approval consumption caller is not allowed")
	}
	if s.approvals == nil || strings.TrimSpace(request.GetContext().GetPrincipalUserId()) != "" || request.GetMode() != "active" {
		return nil, status.Error(codes.InvalidArgument, "Agent Approval consumption is invalid")
	}
	err = s.approvals.Consume(ctx, application.AgentApprovalConsumptionV1{
		TaskUUID: request.GetTaskId(), RunUUID: request.GetRunId(), RuntimeID: "dipole-agent", Mode: request.GetMode(), ApprovalUUID: request.GetApprovalId(),
		Claim: application.AgentApprovalClaimV1{
			TaskUUID: request.GetTaskId(), CapabilityID: request.GetCapabilityId(), ScopeSHA256: request.GetScopeSha256(),
			ArgumentsSHA256: request.GetArgumentsSha256(), NonceSHA256: request.GetNonceSha256(),
		},
	})
	if err != nil {
		return nil, mapApprovalError(err)
	}
	return &agentv1.ConsumeApprovalResponse{ApprovalId: request.GetApprovalId(), Status: string(application.AgentApprovalStatusConsumed)}, nil
}

func (s *Server) ResolveApprovalGrant(ctx context.Context, request *agentv1.ResolveApprovalGrantRequest) (*agentv1.ResolveApprovalGrantResponse, error) {
	if err := s.authorizeMcpToolAuditCallerV1(ctx, request.GetContext()); err != nil {
		return nil, err
	}
	if s.approvalGrants == nil {
		return nil, status.Error(codes.Unavailable, "Agent Approval grant resolver is unavailable")
	}
	if request.GetResourceScope() == nil {
		return nil, status.Error(codes.InvalidArgument, "Agent Approval grant request is invalid")
	}
	grant, err := s.approvalGrants.ResolveGrant(grpccommon.Correlation(ctx, request.GetContext()), application.AgentApprovalGrantRequestV1{
		TaskUUID: request.GetTaskId(), RunUUID: request.GetRunId(), RuntimeID: "dipole-agent", Mode: "active",
		CapabilityID: request.GetCapabilityId(), ArgumentsSHA256: request.GetArgumentsSha256(),
		ResourceScope: application.AgentResourceScopeV1{
			ResourceType: request.GetResourceScope().GetResourceType(), ResourceID: request.GetResourceScope().GetResourceId(),
			Actions: append([]string(nil), request.GetResourceScope().GetActions()...),
		},
	})
	if err != nil {
		if errors.Is(err, application.ErrAgentApprovalDenied) {
			return nil, status.Error(codes.NotFound, "Agent Approval grant is unavailable")
		}
		return nil, status.Error(codes.Internal, "Agent Approval grant lookup failed")
	}
	return &agentv1.ResolveApprovalGrantResponse{
		ApprovalId: grant.ApprovalUUID, CapabilityId: grant.CapabilityID,
		ResourceScope: &agentv1.AgentResourceScope{ResourceType: grant.ResourceScope.ResourceType, ResourceId: grant.ResourceScope.ResourceID, Actions: append([]string(nil), grant.ResourceScope.Actions...)},
		ScopeSha256:   grant.ScopeSHA256, ArgumentsSha256: grant.ArgumentsSHA256, NonceSha256: grant.NonceSHA256,
		ExpiresAtUnixMs: grant.ExpiresAt.UnixMilli(),
	}, nil
}

func approvalResponse(approval *application.AgentApprovalV1) *agentv1.ApprovalResponse {
	return &agentv1.ApprovalResponse{ApprovalId: approval.ApprovalUUID, Status: string(approval.Status), ApprovedByUserId: approval.ApprovedByUUID}
}

func mapApprovalError(err error) error {
	if errors.Is(err, application.ErrAgentApprovalDenied) {
		return status.Error(codes.PermissionDenied, "Agent Approval denied")
	}
	return status.Error(codes.Internal, "Agent Approval transition failed")
}

func (s *Server) AdmitRun(ctx context.Context, request *agentv1.AdmitRunRequest) (*agentv1.AdmitRunResponse, error) {
	if _, err := grpccommon.Caller(ctx, request.GetContext()); err != nil {
		return nil, err
	}
	if strings.TrimSpace(request.GetContext().GetPrincipalUserId()) != "" {
		return nil, status.Error(codes.InvalidArgument, "admission principal belongs to the trusted event payload")
	}
	if strings.TrimSpace(request.GetRuntimeId()) != "dipole-agent" || strings.TrimSpace(request.GetMode()) != "shadow" {
		return nil, status.Error(codes.InvalidArgument, "Agent Run identity is fixed by the authenticated endpoint")
	}
	execution, err := s.admission.Admit(ctx, application.AgentRunAdmissionRequestV1{
		AgentExecutionPolicyStartV1: application.AgentExecutionPolicyStartV1{
			TenantID: request.GetTenantId(), PrincipalUUID: request.GetPrincipalUserId(), AgentUUID: request.GetAgentId(),
			DelegatedByUUID: request.GetPrincipalUserId(), TriggerType: request.GetTriggerType(), TriggerRef: request.GetTriggerRef(),
			SubscriptionUUID: request.GetSubscriptionId(),
			RequestID:        request.GetContext().GetRequestId(), TraceID: request.GetContext().GetTraceId(), EventID: request.GetEventId(),
		}, RuntimeID: "dipole-agent", Mode: "shadow",
	})
	if err != nil {
		if errors.Is(err, application.ErrAgentExecutionPolicyDenied) {
			return nil, status.Error(codes.PermissionDenied, "Agent Run admission denied")
		}
		return nil, status.Error(codes.Internal, "Agent Run admission failed")
	}
	return &agentv1.AdmitRunResponse{TaskId: execution.TaskUUID, RunId: execution.RunUUID, RunStatus: string(execution.RunStatus)}, nil
}

func (s *Server) CompleteRun(ctx context.Context, request *agentv1.CompleteRunRequest) (*agentv1.CompleteRunResponse, error) {
	if _, err := grpccommon.Caller(ctx, request.GetContext()); err != nil {
		return nil, err
	}
	if strings.TrimSpace(request.GetContext().GetPrincipalUserId()) != "" {
		return nil, status.Error(codes.InvalidArgument, "Agent principal must be resolved from Task")
	}
	if err := s.admission.Finish(ctx, request.GetTaskId(), request.GetRunId(), "dipole-agent", "shadow", application.AgentRunStatusCompleted, ""); err != nil {
		if errors.Is(err, application.ErrAgentExecutionPolicyDenied) {
			return nil, status.Error(codes.PermissionDenied, "Agent Run completion denied")
		}
		return nil, status.Error(codes.Internal, "Agent Run completion failed")
	}
	return &agentv1.CompleteRunResponse{RunStatus: string(application.AgentRunStatusCompleted)}, nil
}

func (s *Server) FinishRun(ctx context.Context, request *agentv1.FinishRunRequest) (*agentv1.FinishRunResponse, error) {
	if _, err := grpccommon.Caller(ctx, request.GetContext()); err != nil {
		return nil, err
	}
	if strings.TrimSpace(request.GetContext().GetPrincipalUserId()) != "" {
		return nil, status.Error(codes.InvalidArgument, "Agent principal must be resolved from Task")
	}
	runStatus := application.AgentRunStatusV1(strings.TrimSpace(request.GetRunStatus()))
	lastError := strings.TrimSpace(request.GetLastError())
	if err := application.ValidateAgentRunTerminalV1(runStatus, lastError); err != nil {
		return nil, status.Error(codes.InvalidArgument, "Agent Run terminal evidence is invalid")
	}
	if err := s.admission.Finish(ctx, request.GetTaskId(), request.GetRunId(), "dipole-agent", "shadow", runStatus, lastError); err != nil {
		if errors.Is(err, application.ErrAgentExecutionPolicyDenied) {
			return nil, status.Error(codes.PermissionDenied, "Agent Run terminal transition denied")
		}
		return nil, status.Error(codes.Internal, "Agent Run terminal transition failed")
	}
	return &agentv1.FinishRunResponse{RunStatus: string(runStatus)}, nil
}

func (s *Server) ListConversations(ctx context.Context, request *agentv1.ListConversationsRequest) (*agentv1.ListConversationsResponse, error) {
	if _, err := grpccommon.Caller(ctx, request.GetContext()); err != nil {
		return nil, err
	}
	if strings.TrimSpace(request.GetContext().GetPrincipalUserId()) != "" {
		return nil, status.Error(codes.InvalidArgument, "Agent principal must be resolved from Task")
	}
	limit := int(request.GetLimit())
	if limit < 1 || limit > 100 {
		return nil, status.Error(codes.InvalidArgument, "limit must be between 1 and 100")
	}
	invocation, err := s.resolver.Resolve(ctx, request.GetTaskId(), request.GetRunId())
	if err != nil {
		if errors.Is(err, application.ErrAgentExecutionPolicyDenied) {
			return nil, status.Error(codes.PermissionDenied, "Agent Task policy denied")
		}
		return nil, status.Error(codes.Internal, "Agent Task policy lookup failed")
	}
	items, err := s.capability.ListConversations(ctx, invocation, limit)
	if err != nil {
		if errors.Is(err, application.ErrAgentCapabilityDenied) {
			return nil, status.Error(codes.PermissionDenied, "Agent Capability denied")
		}
		return nil, status.Error(codes.Internal, "Agent conversation list failed")
	}
	response := &agentv1.ListConversationsResponse{Conversations: make([]*agentv1.ConversationSnapshot, 0, len(items))}
	for _, item := range items {
		if item != nil {
			response.Conversations = append(response.Conversations, conversationToProto(item))
		}
	}
	return response, nil
}

func conversationToProto(item *model.Conversation) *agentv1.ConversationSnapshot {
	return &agentv1.ConversationSnapshot{
		ConversationKey: item.ConversationKey, TargetId: item.TargetUUID, TargetType: int32(item.TargetType),
		LastMessageId: item.LastMessageUUID, LastMessageSeq: item.LastMessageSeq,
		LastMessagePreview: item.LastMessagePreview, LastMessageAtUnixMs: item.LastMessageAt.UnixMilli(),
		ReadSeq: item.ReadSeq, UnreadCount: int32(item.UnreadCount),
	}
}
