package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/platform/correlation"
)

const maxAgentControlResponseBytes = 1 << 20

type AgentTaskControlResult struct {
	StatusCode  int
	Body        []byte
	ContentType string
}

type AgentTaskControlApplication interface {
	StartTask(ctx context.Context, principalUUID, clientRequestID, goal string) (*AgentTaskControlResult, error)
	GetTask(ctx context.Context, principalUUID, taskUUID string) (*AgentTaskControlResult, error)
	GetTimeline(ctx context.Context, principalUUID, taskUUID, after string, limit int) (*AgentTaskControlResult, error)
	CancelTask(ctx context.Context, principalUUID, taskUUID, reason string) (*AgentTaskControlResult, error)
	ResolveApproval(ctx context.Context, principalUUID, taskUUID, approvalUUID, decision string) (*AgentTaskControlResult, error)
	ProvideInput(ctx context.Context, principalUUID, taskUUID, requestUUID string, value any) (*AgentTaskControlResult, error)
}

type AgentTaskControlClient struct {
	baseURL *url.URL
	secret  string
	client  *http.Client
}

func NewAgentTaskControlClient(target, secret string, timeout time.Duration) (*AgentTaskControlClient, error) {
	parsed, err := url.Parse(strings.TrimSpace(target))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, errors.New("Agent Task control target is invalid")
	}
	if strings.TrimSpace(secret) == "" {
		return nil, errors.New("Agent Task control secret is required")
	}
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &AgentTaskControlClient{baseURL: parsed, secret: secret, client: &http.Client{Timeout: timeout}}, nil
}

func (c *AgentTaskControlClient) StartTask(ctx context.Context, principalUUID, clientRequestID, goal string) (*AgentTaskControlResult, error) {
	return c.request(ctx, http.MethodPost, principalUUID, "/internal/v1/agent/tasks", map[string]string{
		"clientRequestId": clientRequestID,
		"goal":            goal,
	})
}

func (c *AgentTaskControlClient) GetTask(ctx context.Context, principalUUID, taskUUID string) (*AgentTaskControlResult, error) {
	return c.request(ctx, http.MethodGet, principalUUID, "/internal/v1/agent/tasks/"+url.PathEscape(taskUUID), nil)
}

func (c *AgentTaskControlClient) GetTimeline(ctx context.Context, principalUUID, taskUUID, after string, limit int) (*AgentTaskControlResult, error) {
	path := "/internal/v1/agent/tasks/" + url.PathEscape(taskUUID) + "/timeline?limit=" + url.QueryEscape(fmt.Sprintf("%d", limit))
	if after != "" {
		path += "&after=" + url.QueryEscape(after)
	}
	return c.request(ctx, http.MethodGet, principalUUID, path, nil)
}

func (c *AgentTaskControlClient) CancelTask(ctx context.Context, principalUUID, taskUUID, reason string) (*AgentTaskControlResult, error) {
	return c.request(ctx, http.MethodPost, principalUUID, "/internal/v1/agent/tasks/"+url.PathEscape(taskUUID)+"/cancel", map[string]string{"reason": reason})
}

func (c *AgentTaskControlClient) ResolveApproval(ctx context.Context, principalUUID, taskUUID, approvalUUID, decision string) (*AgentTaskControlResult, error) {
	return c.request(ctx, http.MethodPost, principalUUID, "/internal/v1/agent/tasks/"+url.PathEscape(taskUUID)+"/approvals/"+url.PathEscape(approvalUUID), map[string]string{"decision": decision})
}

func (c *AgentTaskControlClient) ProvideInput(ctx context.Context, principalUUID, taskUUID, requestUUID string, value any) (*AgentTaskControlResult, error) {
	return c.request(ctx, http.MethodPost, principalUUID, "/internal/v1/agent/tasks/"+url.PathEscape(taskUUID)+"/inputs/"+url.PathEscape(requestUUID), map[string]any{"value": value})
}

func (c *AgentTaskControlClient) request(ctx context.Context, method, principalUUID, path string, payload any) (*AgentTaskControlResult, error) {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("encode Agent Task control request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}
	target := c.baseURL.ResolveReference(&url.URL{Path: path})
	request, err := http.NewRequestWithContext(ctx, method, target.String(), body)
	if err != nil {
		return nil, fmt.Errorf("create Agent Task control request: %w", err)
	}
	request.Header.Set("X-Dipole-Caller-Service", "dipole-gateway")
	request.Header.Set("X-Dipole-Service-Token", c.secret)
	request.Header.Set("X-Dipole-Principal-User-ID", principalUUID)
	request.Header.Set("Content-Type", "application/json")
	ids := correlation.FromContext(ctx)
	request.Header.Set(correlation.RequestHeader, ids.RequestID)
	request.Header.Set(correlation.TraceHeader, ids.TraceID)
	response, err := c.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call Agent Task control runtime: %w", err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxAgentControlResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read Agent Task control response: %w", err)
	}
	if len(responseBody) > maxAgentControlResponseBytes {
		return nil, errors.New("Agent Task control response exceeds limit")
	}
	return &AgentTaskControlResult{StatusCode: response.StatusCode, Body: responseBody, ContentType: response.Header.Get("Content-Type")}, nil
}
