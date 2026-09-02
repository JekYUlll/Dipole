package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/platform/correlation"
)

const agentOAuthCallbackHandoffNotifyPath = "/internal/v1/agent/oauth/callback-handoffs"

var (
	ErrAgentOAuthCallbackHandoffNotifierInvalid     = errors.New("Agent OAuth callback handoff notifier is invalid")
	ErrAgentOAuthCallbackHandoffNotifierUnavailable = errors.New("Agent OAuth callback handoff notifier is unavailable")
)

// AgentOAuthCallbackHandoffNotifier owns the Gateway-to-Runtime control
// notification. Its payload intentionally contains only the opaque handoff ID.
type AgentOAuthCallbackHandoffNotifier struct {
	baseURL *url.URL
	secret  string
	client  *http.Client
}

func NewAgentOAuthCallbackHandoffNotifier(target, secret string, timeout time.Duration) (*AgentOAuthCallbackHandoffNotifier, error) {
	parsed, err := url.Parse(strings.TrimSpace(target))
	if err != nil || !validAgentOAuthCallbackHandoffNotifyTarget(parsed) || strings.TrimSpace(secret) == "" {
		return nil, ErrAgentOAuthCallbackHandoffNotifierInvalid
	}
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &AgentOAuthCallbackHandoffNotifier{baseURL: parsed, secret: secret, client: &http.Client{Timeout: timeout}}, nil
}

func (n *AgentOAuthCallbackHandoffNotifier) Notify(ctx context.Context, handoffID string) error {
	if n == nil || !validOAuthTransactionID(handoffID) {
		return ErrAgentOAuthCallbackHandoffNotifierInvalid
	}
	body, err := json.Marshal(struct {
		HandoffID string `json:"handoff_id"`
	}{HandoffID: handoffID})
	if err != nil {
		return ErrAgentOAuthCallbackHandoffNotifierInvalid
	}
	target := n.baseURL.ResolveReference(&url.URL{Path: agentOAuthCallbackHandoffNotifyPath})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, target.String(), bytes.NewReader(body))
	if err != nil {
		return ErrAgentOAuthCallbackHandoffNotifierUnavailable
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Dipole-Caller-Service", "dipole-gateway")
	request.Header.Set("X-Dipole-Service-Token", n.secret)
	ids := correlation.FromContext(ctx)
	request.Header.Set(correlation.RequestHeader, ids.RequestID)
	request.Header.Set(correlation.TraceHeader, ids.TraceID)
	response, err := n.client.Do(request)
	if err != nil {
		return ErrAgentOAuthCallbackHandoffNotifierUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		return ErrAgentOAuthCallbackHandoffNotifierUnavailable
	}
	return nil
}

func validAgentOAuthCallbackHandoffNotifyTarget(target *url.URL) bool {
	if target == nil || target.Host == "" || target.User != nil || target.RawQuery != "" || target.Fragment != "" {
		return false
	}
	if target.Scheme == "https" {
		return true
	}
	if target.Scheme != "http" {
		return false
	}
	host := strings.Trim(strings.ToLower(target.Hostname()), "[]")
	if host == "localhost" {
		return true
	}
	parsed := net.ParseIP(host)
	return parsed != nil && parsed.IsLoopback()
}
