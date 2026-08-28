package application

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
)

const agentMCPToolRoundResultLimitV1 = 128 * 1024

var (
	ErrAgentMCPToolRoundInvalid  = errors.New("agent MCP Tool round invalid")
	ErrAgentMCPToolRoundDenied   = errors.New("agent MCP Tool round denied")
	ErrAgentMCPToolRoundConflict = errors.New("agent MCP Tool round conflict")
)

type AgentMCPToolRoundStatusV1 string
type AgentMCPToolRoundClaimOutcomeV1 string

const (
	AgentMCPToolRoundStatusExecuting AgentMCPToolRoundStatusV1 = "executing"
	AgentMCPToolRoundStatusCompleted AgentMCPToolRoundStatusV1 = "completed"
	AgentMCPToolRoundStatusFailed    AgentMCPToolRoundStatusV1 = "failed"

	AgentMCPToolRoundClaimed         AgentMCPToolRoundClaimOutcomeV1 = "claimed"
	AgentMCPToolRoundReplayCompleted AgentMCPToolRoundClaimOutcomeV1 = "replay_completed"
	AgentMCPToolRoundReplayFailed    AgentMCPToolRoundClaimOutcomeV1 = "replay_failed"
	AgentMCPToolRoundAmbiguous       AgentMCPToolRoundClaimOutcomeV1 = "ambiguous"
)

type AgentMCPToolRoundClaimV1 struct {
	RoundUUID        string
	InvocationUUID   string
	TaskUUID         string
	RunUUID          string
	RoundNumber      uint8
	RequestSHA256    string
	OwnerTokenSHA256 string
}

type AgentMCPToolRoundFinishV1 struct {
	RoundUUID        string
	OwnerTokenSHA256 string
	Status           AgentMCPToolRoundStatusV1
	ResultJSON       string
	ResultSHA256     string
	ErrorCode        string
}

type AgentMCPToolRoundV1 struct {
	AgentMCPToolRoundClaimV1
	Status       AgentMCPToolRoundStatusV1
	ResultJSON   string
	ResultSHA256 string
	ErrorCode    string
}

type AgentMCPToolRoundClaimResultV1 struct {
	Outcome      AgentMCPToolRoundClaimOutcomeV1
	ResultJSON   string
	ResultSHA256 string
	ErrorCode    string
}

type AgentMCPToolInvocationTerminalRequestV1 struct {
	TaskUUID       string
	RunUUID        string
	InvocationUUID string
	RoundUUID      string
}

type AgentMCPToolRoundStoreV1 interface {
	ClaimMCPToolRound(ctx context.Context, claim AgentMCPToolRoundClaimV1) (bool, error)
	GetMCPToolRound(ctx context.Context, roundUUID string) (*AgentMCPToolRoundV1, error)
	FinishMCPToolRound(ctx context.Context, finish AgentMCPToolRoundFinishV1) (bool, error)
}

type AgentMCPToolRoundServiceV1 interface {
	Claim(ctx context.Context, claim AgentMCPToolRoundClaimV1) (*AgentMCPToolRoundClaimResultV1, error)
	Finish(ctx context.Context, finish AgentMCPToolRoundFinishV1) error
}

type AgentMCPToolInvocationTerminalServiceV1 interface {
	FinishFromRound(ctx context.Context, request AgentMCPToolInvocationTerminalRequestV1) (*AgentToolInvocationV1, error)
}

var agentMCPToolRoundSHA256PatternV1 = regexp.MustCompile(`^[a-f0-9]{64}$`)
var agentMCPToolRoundErrorCodePatternV1 = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

func (v AgentMCPToolRoundClaimV1) Validate() error {
	if !agentMCPToolRoundSHA256PatternV1.MatchString(v.RoundUUID) ||
		!validExactAgentMCPToolRoundIdentifierV1(v.InvocationUUID, 64) ||
		!validExactAgentMCPToolRoundIdentifierV1(v.TaskUUID, 64) ||
		!validExactAgentMCPToolRoundIdentifierV1(v.RunUUID, 64) ||
		v.RoundNumber > 1 ||
		!agentMCPToolRoundSHA256PatternV1.MatchString(v.RequestSHA256) ||
		!agentMCPToolRoundSHA256PatternV1.MatchString(v.OwnerTokenSHA256) {
		return ErrAgentMCPToolRoundInvalid
	}
	return nil
}

func (v AgentMCPToolRoundFinishV1) Validate() error {
	if !agentMCPToolRoundSHA256PatternV1.MatchString(v.RoundUUID) ||
		!agentMCPToolRoundSHA256PatternV1.MatchString(v.OwnerTokenSHA256) {
		return ErrAgentMCPToolRoundInvalid
	}
	switch v.Status {
	case AgentMCPToolRoundStatusCompleted:
		if v.ErrorCode != "" || !agentMCPToolRoundSHA256PatternV1.MatchString(v.ResultSHA256) ||
			len(v.ResultJSON) == 0 || len(v.ResultJSON) > agentMCPToolRoundResultLimitV1 || !canonicalAgentMCPToolRoundResultV1(v.ResultJSON) ||
			fmt.Sprintf("%x", sha256.Sum256([]byte(v.ResultJSON))) != v.ResultSHA256 {
			return ErrAgentMCPToolRoundInvalid
		}
	case AgentMCPToolRoundStatusFailed:
		if v.ResultJSON != "" || v.ResultSHA256 != "" || !agentMCPToolRoundErrorCodePatternV1.MatchString(v.ErrorCode) {
			return ErrAgentMCPToolRoundInvalid
		}
	default:
		return ErrAgentMCPToolRoundInvalid
	}
	return nil
}

func (v AgentMCPToolRoundClaimOutcomeV1) Valid() bool {
	switch v {
	case AgentMCPToolRoundClaimed, AgentMCPToolRoundReplayCompleted, AgentMCPToolRoundReplayFailed, AgentMCPToolRoundAmbiguous:
		return true
	default:
		return false
	}
}

func (v AgentMCPToolInvocationTerminalRequestV1) Validate() error {
	if !validExactAgentMCPToolRoundIdentifierV1(v.TaskUUID, 64) ||
		!validExactAgentMCPToolRoundIdentifierV1(v.RunUUID, 64) ||
		!validExactAgentMCPToolRoundIdentifierV1(v.InvocationUUID, 64) ||
		!agentMCPToolRoundSHA256PatternV1.MatchString(v.RoundUUID) {
		return ErrAgentMCPToolRoundInvalid
	}
	return nil
}

func canonicalAgentMCPToolRoundResultV1(value string) bool {
	decoder := json.NewDecoder(strings.NewReader(value))
	decoder.UseNumber()
	var decoded map[string]any
	if err := decoder.Decode(&decoded); err != nil || decoded == nil {
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return false
	}
	canonical, err := json.Marshal(decoded)
	return err == nil && string(canonical) == value
}

func validExactAgentMCPToolRoundIdentifierV1(value string, limit int) bool {
	return value == strings.TrimSpace(value) && validAgentToolIdentifierV1(value, limit)
}
