package gateway

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	agentv1 "github.com/JekYUlll/Dipole/api/gen/go/agent/v1"
	"github.com/JekYUlll/Dipole/internal/middleware"
	grpccommon "github.com/JekYUlll/Dipole/internal/transport/grpc/common"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrAgentRuntimePromotionInvalid     = errors.New("Agent Runtime promotion request is invalid")
	ErrAgentRuntimePromotionDenied      = errors.New("Agent Runtime promotion access denied")
	ErrAgentRuntimePromotionConflict    = errors.New("Agent Runtime promotion conflicts")
	ErrAgentRuntimePromotionUnavailable = errors.New("Agent Runtime promotion control is unavailable")
)

type AgentRuntimePromotionProposal struct {
	ProposalID           string `json:"proposalId"`
	TenantID             string `json:"tenantId"`
	RuntimeID            string `json:"runtimeId"`
	CandidateVersion     string `json:"candidateVersion"`
	DefinitionID         string `json:"definitionId"`
	DefinitionVersion    uint64 `json:"definitionVersion"`
	EvidenceArtifactID   string `json:"evidenceArtifactId"`
	EvidenceSHA256       string `json:"evidenceSha256"`
	EvalSuiteSHA256      string `json:"evalSuiteSha256"`
	ProposerID           string `json:"proposerId"`
	TicketRef            string `json:"ticketRef"`
	Reason               string `json:"reason"`
	Status               string `json:"status"`
	GrantID              string `json:"grantId,omitempty"`
	ProposedAtUnixMS     int64  `json:"proposedAtUnixMs"`
	ExpiresAtUnixMS      int64  `json:"expiresAtUnixMs"`
	GrantValidFromUnixMS int64  `json:"grantValidFromUnixMs"`
	GrantExpiresAtUnixMS int64  `json:"grantExpiresAtUnixMs"`
	DecidedAtUnixMS      int64  `json:"decidedAtUnixMs,omitempty"`
}

type AgentRuntimePromotionProposeInput struct {
	RuntimeID            string `json:"runtimeId"`
	CandidateVersion     string `json:"candidateVersion"`
	DefinitionID         string `json:"definitionId"`
	DefinitionVersion    uint64 `json:"definitionVersion"`
	EvidenceArtifactID   string `json:"evidenceArtifactId"`
	EvidenceSHA256       string `json:"evidenceSha256"`
	EvalSuiteSHA256      string `json:"evalSuiteSha256"`
	TicketRef            string `json:"ticketRef"`
	Reason               string `json:"reason"`
	ExpiresAtUnixMS      int64  `json:"expiresAtUnixMs"`
	GrantValidFromUnixMS int64  `json:"grantValidFromUnixMs"`
	GrantExpiresAtUnixMS int64  `json:"grantExpiresAtUnixMs"`
}

type AgentRuntimePromotionApplication interface {
	Propose(context.Context, string, AgentRuntimePromotionProposeInput) (*AgentRuntimePromotionProposal, error)
	Get(context.Context, string, string) (*AgentRuntimePromotionProposal, error)
	Review(context.Context, string, string, string) (*AgentRuntimePromotionProposal, error)
	Revoke(context.Context, string, string, string, string) (*AgentRuntimePromotionGrant, error)
}

type AgentRuntimePromotionGrant struct {
	GrantID           string `json:"grantId"`
	TenantID          string `json:"tenantId"`
	RuntimeID         string `json:"runtimeId"`
	CandidateVersion  string `json:"candidateVersion"`
	DefinitionID      string `json:"definitionId"`
	DefinitionVersion uint64 `json:"definitionVersion"`
	PolicyVersion     string `json:"policyVersion"`
	EvidenceSHA256    string `json:"evidenceSha256"`
	EvalSuiteSHA256   string `json:"evalSuiteSha256"`
	GrantedByID       string `json:"grantedById"`
	ReviewedByID      string `json:"reviewedById"`
	ValidFromUnixMS   int64  `json:"validFromUnixMs"`
	ExpiresAtUnixMS   int64  `json:"expiresAtUnixMs"`
	RevokedAtUnixMS   int64  `json:"revokedAtUnixMs"`
}

type agentRuntimePromotionRPC interface {
	ProposeRuntimePromotion(context.Context, *agentv1.ProposeRuntimePromotionRequest, ...grpc.CallOption) (*agentv1.RuntimePromotionProposalResponse, error)
	GetRuntimePromotion(context.Context, *agentv1.GetRuntimePromotionRequest, ...grpc.CallOption) (*agentv1.RuntimePromotionProposalResponse, error)
	ReviewRuntimePromotion(context.Context, *agentv1.ReviewRuntimePromotionRequest, ...grpc.CallOption) (*agentv1.RuntimePromotionProposalResponse, error)
	RevokeRuntimePromotion(context.Context, *agentv1.RevokeRuntimePromotionRequest, ...grpc.CallOption) (*agentv1.RuntimePromotionGrantResponse, error)
}

type AgentRuntimePromotionClient struct {
	rpc      agentRuntimePromotionRPC
	tenantID string
	timeout  time.Duration
}

func NewAgentRuntimePromotionClient(rpc agentRuntimePromotionRPC, tenantID string, timeout time.Duration) (*AgentRuntimePromotionClient, error) {
	tenantID = strings.TrimSpace(tenantID)
	if rpc == nil || !validAgentSubscriptionPublicID(tenantID, 64) {
		return nil, ErrAgentRuntimePromotionInvalid
	}
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &AgentRuntimePromotionClient{rpc: rpc, tenantID: tenantID, timeout: timeout}, nil
}

func (c *AgentRuntimePromotionClient) Propose(ctx context.Context, principal string, input AgentRuntimePromotionProposeInput) (*AgentRuntimePromotionProposal, error) {
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	response, err := c.rpc.ProposeRuntimePromotion(callCtx, &agentv1.ProposeRuntimePromotionRequest{
		Context: grpccommon.RequestContextFrom(ctx, principal, "dipole-gateway"), TenantId: c.tenantID,
		RuntimeId: input.RuntimeID, CandidateVersion: input.CandidateVersion, DefinitionId: input.DefinitionID,
		DefinitionVersion: input.DefinitionVersion, EvidenceArtifactId: input.EvidenceArtifactID,
		EvidenceSha256: input.EvidenceSHA256, EvalSuiteSha256: input.EvalSuiteSHA256, TicketRef: input.TicketRef,
		Reason: input.Reason, ProposedAtUnixMs: time.Now().UTC().UnixMilli(), ExpiresAtUnixMs: input.ExpiresAtUnixMS,
		GrantValidFromUnixMs: input.GrantValidFromUnixMS, GrantExpiresAtUnixMs: input.GrantExpiresAtUnixMS,
	})
	if err != nil {
		return nil, mapAgentRuntimePromotionRPCError(err)
	}
	return agentRuntimePromotionProposalFromProto(response, c.tenantID)
}

func (c *AgentRuntimePromotionClient) Get(ctx context.Context, principal, proposalID string) (*AgentRuntimePromotionProposal, error) {
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	response, err := c.rpc.GetRuntimePromotion(callCtx, &agentv1.GetRuntimePromotionRequest{Context: grpccommon.RequestContextFrom(ctx, principal, "dipole-gateway"), TenantId: c.tenantID, ProposalId: proposalID})
	if err != nil {
		return nil, mapAgentRuntimePromotionRPCError(err)
	}
	return agentRuntimePromotionProposalFromProto(response, c.tenantID)
}

func (c *AgentRuntimePromotionClient) Review(ctx context.Context, principal, proposalID, decision string) (*AgentRuntimePromotionProposal, error) {
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	response, err := c.rpc.ReviewRuntimePromotion(callCtx, &agentv1.ReviewRuntimePromotionRequest{Context: grpccommon.RequestContextFrom(ctx, principal, "dipole-gateway"), ProposalId: proposalID, Decision: decision})
	if err != nil {
		return nil, mapAgentRuntimePromotionRPCError(err)
	}
	return agentRuntimePromotionProposalFromProto(response, c.tenantID)
}

func (c *AgentRuntimePromotionClient) Revoke(ctx context.Context, principal, grantID, ticketRef, reason string) (*AgentRuntimePromotionGrant, error) {
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	response, err := c.rpc.RevokeRuntimePromotion(callCtx, &agentv1.RevokeRuntimePromotionRequest{Context: grpccommon.RequestContextFrom(ctx, principal, "dipole-gateway"), GrantId: grantID, TicketRef: ticketRef, Reason: reason})
	if err != nil {
		return nil, mapAgentRuntimePromotionRPCError(err)
	}
	if response == nil || !validAgentSubscriptionPublicID(response.GetGrantId(), 64) || response.GetTenantId() != c.tenantID || response.GetRevokedAtUnixMs() <= 0 {
		return nil, ErrAgentRuntimePromotionUnavailable
	}
	return &AgentRuntimePromotionGrant{GrantID: response.GetGrantId(), TenantID: response.GetTenantId(), RuntimeID: response.GetRuntimeId(), CandidateVersion: response.GetCandidateVersion(), DefinitionID: response.GetDefinitionId(), DefinitionVersion: response.GetDefinitionVersion(), PolicyVersion: response.GetPolicyVersion(), EvidenceSHA256: response.GetEvidenceSha256(), EvalSuiteSHA256: response.GetEvalSuiteSha256(), GrantedByID: response.GetGrantedById(), ReviewedByID: response.GetReviewedById(), ValidFromUnixMS: response.GetValidFromUnixMs(), ExpiresAtUnixMS: response.GetExpiresAtUnixMs(), RevokedAtUnixMS: response.GetRevokedAtUnixMs()}, nil
}

func agentRuntimePromotionProposalFromProto(value *agentv1.RuntimePromotionProposalResponse, tenantID string) (*AgentRuntimePromotionProposal, error) {
	if value == nil || value.GetTenantId() != tenantID || !validAgentSubscriptionPublicID(value.GetProposalId(), 64) || !validAgentSubscriptionPublicID(value.GetRuntimeId(), 64) || !validAgentSubscriptionPublicID(value.GetCandidateVersion(), 128) || !validAgentSubscriptionPublicID(value.GetDefinitionId(), 64) || value.GetDefinitionVersion() == 0 || (value.GetStatus() != "proposed" && value.GetStatus() != "approved" && value.GetStatus() != "rejected") || value.GetProposedAtUnixMs() <= 0 || value.GetExpiresAtUnixMs() <= value.GetProposedAtUnixMs() || value.GetGrantExpiresAtUnixMs() <= value.GetGrantValidFromUnixMs() {
		return nil, ErrAgentRuntimePromotionUnavailable
	}
	if value.GetStatus() == "approved" && !validAgentSubscriptionPublicID(value.GetGrantId(), 64) {
		return nil, ErrAgentRuntimePromotionUnavailable
	}
	return &AgentRuntimePromotionProposal{ProposalID: value.GetProposalId(), TenantID: value.GetTenantId(), RuntimeID: value.GetRuntimeId(), CandidateVersion: value.GetCandidateVersion(), DefinitionID: value.GetDefinitionId(), DefinitionVersion: value.GetDefinitionVersion(), EvidenceArtifactID: value.GetEvidenceArtifactId(), EvidenceSHA256: value.GetEvidenceSha256(), EvalSuiteSHA256: value.GetEvalSuiteSha256(), ProposerID: value.GetProposerId(), TicketRef: value.GetTicketRef(), Reason: value.GetReason(), Status: value.GetStatus(), GrantID: value.GetGrantId(), ProposedAtUnixMS: value.GetProposedAtUnixMs(), ExpiresAtUnixMS: value.GetExpiresAtUnixMs(), GrantValidFromUnixMS: value.GetGrantValidFromUnixMs(), GrantExpiresAtUnixMS: value.GetGrantExpiresAtUnixMs(), DecidedAtUnixMS: value.GetDecidedAtUnixMs()}, nil
}

func agentRuntimePromotionProposeHandler(promotions AgentRuntimePromotionApplication) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := currentGatewayUser(c)
		if !ok {
			return
		}
		var input AgentRuntimePromotionProposeInput
		if err := decodeStrictAgentSubscriptionBody(c.Request.Body, &input); err != nil || !validAgentRuntimePromotionInput(input) {
			writeAgentRuntimePromotionResult(c, nil, ErrAgentRuntimePromotionInvalid)
			return
		}
		result, err := promotions.Propose(c.Request.Context(), user, input)
		writeAgentRuntimePromotionResult(c, result, err)
	}
}

func agentRuntimePromotionGetHandler(promotions AgentRuntimePromotionApplication) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := currentGatewayUser(c)
		if !ok {
			return
		}
		proposalID := strings.TrimSpace(c.Param("proposal_id"))
		if !validAgentSubscriptionPublicID(proposalID, 64) {
			writeAgentRuntimePromotionResult(c, nil, ErrAgentRuntimePromotionInvalid)
			return
		}
		result, err := promotions.Get(c.Request.Context(), user, proposalID)
		writeAgentRuntimePromotionResult(c, result, err)
	}
}

func agentRuntimePromotionReviewHandler(promotions AgentRuntimePromotionApplication) gin.HandlerFunc {
	type reviewInput struct {
		Decision string `json:"decision"`
	}
	return func(c *gin.Context) {
		user, ok := currentGatewayUser(c)
		if !ok {
			return
		}
		proposalID := strings.TrimSpace(c.Param("proposal_id"))
		var input reviewInput
		if !validAgentSubscriptionPublicID(proposalID, 64) || decodeStrictAgentSubscriptionBody(c.Request.Body, &input) != nil || (input.Decision != "approved" && input.Decision != "rejected") {
			writeAgentRuntimePromotionResult(c, nil, ErrAgentRuntimePromotionInvalid)
			return
		}
		result, err := promotions.Review(c.Request.Context(), user, proposalID, input.Decision)
		writeAgentRuntimePromotionResult(c, result, err)
	}
}

func agentRuntimePromotionRevokeHandler(promotions AgentRuntimePromotionApplication) gin.HandlerFunc {
	type revokeInput struct {
		TicketRef string `json:"ticketRef"`
		Reason    string `json:"reason"`
	}
	return func(c *gin.Context) {
		user, ok := currentGatewayUser(c)
		if !ok {
			return
		}
		grantID := strings.TrimSpace(c.Param("grant_id"))
		var input revokeInput
		if !validAgentSubscriptionPublicID(grantID, 64) || decodeStrictAgentSubscriptionBody(c.Request.Body, &input) != nil || !validAgentRuntimePromotionText(input.TicketRef, 128) || !validAgentRuntimePromotionText(input.Reason, 1000) {
			writeAgentRuntimePromotionResult(c, nil, ErrAgentRuntimePromotionInvalid)
			return
		}
		result, err := promotions.Revoke(c.Request.Context(), user, grantID, strings.TrimSpace(input.TicketRef), strings.TrimSpace(input.Reason))
		writeAgentRuntimePromotionResult(c, result, err)
	}
}

func currentGatewayUser(c *gin.Context) (string, bool) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{"code": 401, "message": "user session is invalid"})
		return "", false
	}
	return user.UUID, true
}

func validAgentRuntimePromotionInput(input AgentRuntimePromotionProposeInput) bool {
	return validAgentSubscriptionPublicID(strings.TrimSpace(input.RuntimeID), 64) && validAgentSubscriptionPublicID(strings.TrimSpace(input.CandidateVersion), 128) && validAgentSubscriptionPublicID(strings.TrimSpace(input.DefinitionID), 64) && input.DefinitionVersion > 0 && validAgentSubscriptionPublicID(strings.TrimSpace(input.EvidenceArtifactID), 64) && validAgentSubscriptionPublicID(strings.TrimSpace(input.EvidenceSHA256), 64) && validAgentSubscriptionPublicID(strings.TrimSpace(input.EvalSuiteSHA256), 64) && validAgentRuntimePromotionText(input.TicketRef, 128) && validAgentRuntimePromotionText(input.Reason, 1000) && input.ExpiresAtUnixMS > 0 && input.GrantValidFromUnixMS > 0 && input.GrantExpiresAtUnixMS > input.GrantValidFromUnixMS
}

func validAgentRuntimePromotionText(value string, maximum int) bool {
	value = strings.TrimSpace(value)
	return value != "" && utf8.RuneCountInString(value) <= maximum && strings.IndexFunc(value, unicode.IsControl) < 0
}

func mapAgentRuntimePromotionRPCError(err error) error {
	switch status.Code(err) {
	case codes.InvalidArgument:
		return ErrAgentRuntimePromotionInvalid
	case codes.FailedPrecondition, codes.Aborted, codes.AlreadyExists:
		return ErrAgentRuntimePromotionConflict
	case codes.Unauthenticated, codes.PermissionDenied, codes.NotFound:
		return ErrAgentRuntimePromotionDenied
	case codes.Unavailable, codes.DeadlineExceeded, codes.Canceled:
		return ErrAgentRuntimePromotionUnavailable
	default:
		return ErrAgentRuntimePromotionUnavailable
	}
}

func writeAgentRuntimePromotionResult(c *gin.Context, value any, err error) {
	if err == nil {
		c.JSON(200, value)
		return
	}
	statusCode := 503
	message := "Agent Runtime promotion control is unavailable"
	switch {
	case errors.Is(err, ErrAgentRuntimePromotionInvalid):
		statusCode, message = 400, "Agent Runtime promotion request is invalid"
	case errors.Is(err, ErrAgentRuntimePromotionDenied):
		statusCode, message = 403, "Agent Runtime promotion request denied"
	case errors.Is(err, ErrAgentRuntimePromotionConflict):
		statusCode, message = 409, "Agent Runtime promotion request conflicts"
	}
	c.JSON(statusCode, gin.H{"code": statusCode, "message": message})
}
