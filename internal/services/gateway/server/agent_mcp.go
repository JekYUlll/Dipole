package gateway

import (
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"

	"github.com/JekYUlll/Dipole/internal/platform/correlation"
	coreauth "github.com/JekYUlll/Dipole/internal/services/core/domain/auth"
)

var agentMCPIdentifier = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

type AgentMCPApplication interface {
	ServeMCP(http.ResponseWriter, *http.Request, string, string, string)
}

type AgentMCPProxy struct {
	target   *url.URL
	secret   string
	resource string
}

func NewAgentMCPProxy(target, secret, resource string) (*AgentMCPProxy, error) {
	parsed, err := url.Parse(strings.TrimSpace(target))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, errors.New("Agent MCP target is invalid")
	}
	if strings.TrimSpace(secret) == "" {
		return nil, errors.New("Agent MCP secret is required")
	}
	if err := coreauth.ValidateAgentMCPResource(resource); err != nil {
		return nil, errors.New("Agent MCP resource is invalid")
	}
	return &AgentMCPProxy{target: parsed, secret: secret, resource: strings.TrimSpace(resource)}, nil
}

func (p *AgentMCPProxy) ServeMCP(writer http.ResponseWriter, request *http.Request, principalUUID, taskUUID, runUUID string) {
	if !agentMCPIdentifier.MatchString(principalUUID) || !agentMCPIdentifier.MatchString(taskUUID) || !agentMCPIdentifier.MatchString(runUUID) {
		http.Error(writer, `{"code":400,"message":"invalid Agent MCP identity"}`, http.StatusBadRequest)
		return
	}
	proxy := httputil.NewSingleHostReverseProxy(p.target)
	proxy.FlushInterval = -1
	originalDirector := proxy.Director
	proxy.Director = func(proxied *http.Request) {
		originalDirector(proxied)
		proxied.URL.Path = "/internal/v1/agent/tasks/" + url.PathEscape(taskUUID) + "/runs/" + url.PathEscape(runUUID) + "/mcp"
		proxied.Header.Del("Authorization")
		proxied.Header.Del("Cookie")
		proxied.Header.Del("X-Dipole-Caller-Service")
		proxied.Header.Del("X-Dipole-Service-Token")
		proxied.Header.Del("X-Dipole-Principal-User-ID")
		proxied.Header.Del("X-Dipole-OAuth-Resource")
		proxied.Header.Del("X-Dipole-OAuth-Scope")
		proxied.Header.Set("X-Dipole-Caller-Service", "dipole-gateway")
		proxied.Header.Set("X-Dipole-Service-Token", p.secret)
		proxied.Header.Set("X-Dipole-Principal-User-ID", principalUUID)
		proxied.Header.Set("X-Dipole-OAuth-Resource", p.resource)
		proxied.Header.Set("X-Dipole-OAuth-Scope", coreauth.AgentMCPReadScope)
		ids := correlation.FromContext(proxied.Context())
		proxied.Header.Set(correlation.RequestHeader, ids.RequestID)
		proxied.Header.Set(correlation.TraceHeader, ids.TraceID)
	}
	proxy.ErrorHandler = func(response http.ResponseWriter, _ *http.Request, _ error) {
		response.Header().Set("Content-Type", "application/json")
		response.WriteHeader(http.StatusBadGateway)
		_, _ = response.Write([]byte(`{"code":502,"message":"Agent MCP runtime unavailable"}`))
	}
	proxy.ServeHTTP(writer, request)
}
