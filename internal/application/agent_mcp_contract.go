package application

import (
	"errors"
	"net/url"
	"strings"
	"time"
)

const (
	AgentMCPResource  = "https://dipole.local/api/v1/agent/mcp"
	AgentMCPReadScope = "dipole.agent.mcp.read"
)

var ErrInvalidAgentMCPResource = errors.New("invalid Agent MCP resource")

type AgentTokenSession struct {
	UserUUID  string
	TokenID   string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

type TokenSessionResolver interface {
	ResolveSession(string) (*AgentTokenSession, error)
}

type TokenResolver interface {
	TokenSessionResolver
	Resolve(string) (string, error)
	ResolveAgentMCPAccessToken(string, string, string) (*AgentTokenSession, error)
}

func AgentMCPResourceIdentifier(configured string) string {
	if resource := strings.TrimSpace(configured); resource != "" {
		return resource
	}
	return AgentMCPResource
}

// ValidateAgentMCPResource validates the resource identifier shared by the
// Gateway proxy and the Core token contract.
func ValidateAgentMCPResource(resource string) error {
	parsed, err := url.Parse(strings.TrimSpace(resource))
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" || parsed.Fragment != "" || parsed.User != nil || parsed.RawQuery != "" {
		return ErrInvalidAgentMCPResource
	}
	return nil
}
