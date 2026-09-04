package gateway

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrAgentOAuthCallbackHandlerInvalid = errors.New("Agent OAuth callback handler is invalid")

type agentOAuthCallbackHandoffRecorder interface {
	Record(context.Context, string, AgentOAuthCallbackHandoffRecordInput) (*AgentOAuthCallbackHandoffRecordResult, error)
}

type agentOAuthCallbackHandoffNotifier interface {
	Notify(context.Context, string) error
}

// AgentOAuthCallbackHandlerConfig contains only prevalidated local material.
// The handler remains unmounted until an explicit OAuth deployment profile
// supplies cookie issuance, routing, mTLS and Provider review.
type AgentOAuthCallbackHandlerConfig struct {
	CorrelationSecret                               []byte
	BrowserSessionCookieName, CorrelationCookieName string
	RedirectURI, RuntimeKeyID                       string
	RuntimePublicKeyPEM                             []byte
	Record                                          agentOAuthCallbackHandoffRecorder
	Notify                                          agentOAuthCallbackHandoffNotifier
	Now                                             func() time.Time
	DeriveHandoffID                                 func(AgentOAuthCallbackCorrelationV1, string) (string, error)
}

type AgentOAuthCallbackHandler struct {
	config AgentOAuthCallbackHandlerConfig
}

func NewAgentOAuthCallbackHandler(config AgentOAuthCallbackHandlerConfig) (*AgentOAuthCallbackHandler, error) {
	if len(config.CorrelationSecret) < 32 || strings.TrimSpace(config.BrowserSessionCookieName) == "" ||
		strings.TrimSpace(config.CorrelationCookieName) == "" || !validOAuthURL(config.RedirectURI) ||
		!validAgentOAuthCallbackEnvelopeIdentifier(config.RuntimeKeyID, 128) || config.Record == nil || config.Notify == nil {
		return nil, ErrAgentOAuthCallbackHandlerInvalid
	}
	if _, err := parseAgentOAuthRuntimePublicKey(config.RuntimePublicKeyPEM); err != nil {
		return nil, ErrAgentOAuthCallbackHandlerInvalid
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	if config.DeriveHandoffID == nil {
		config.DeriveHandoffID = func(correlation AgentOAuthCallbackCorrelationV1, codeSHA256 string) (string, error) {
			return deriveAgentOAuthCallbackHandoffID(config.CorrelationSecret, correlation, codeSHA256), nil
		}
	}
	return &AgentOAuthCallbackHandler{config: config}, nil
}

func (h *AgentOAuthCallbackHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "no-store")
	if h == nil || request.Method != http.MethodGet {
		writeAgentOAuthCallbackError(writer, http.StatusBadRequest)
		return
	}
	correlation, code, err := h.validate(request)
	if err != nil {
		writeAgentOAuthCallbackError(writer, http.StatusBadRequest)
		return
	}
	codeDigest := sha256Hex(code)
	handoffID, err := h.config.DeriveHandoffID(correlation, codeDigest)
	if err != nil || !validOAuthTransactionID(handoffID) {
		writeAgentOAuthCallbackError(writer, http.StatusServiceUnavailable)
		return
	}
	sealed, err := SealAgentOAuthCallbackCode(AgentOAuthCallbackEnvelopeInput{
		HandoffUUID: handoffID, TransactionUUID: correlation.TransactionID, OwnerUserUUID: correlation.OwnerUserID,
		Issuer: correlation.Issuer, RedirectURI: correlation.RedirectURI, AuthorizationCode: code,
		AuthorizationCodeSHA256: codeDigest, RuntimeKeyID: h.config.RuntimeKeyID,
		ExpiresAtRFC3339Millis: correlation.ExpiresAt.UTC().Format("2006-01-02T15:04:05.000Z"),
	}, h.config.RuntimePublicKeyPEM)
	if err != nil {
		writeAgentOAuthCallbackError(writer, http.StatusServiceUnavailable)
		return
	}
	if _, err = h.config.Record.Record(request.Context(), correlation.OwnerUserID, AgentOAuthCallbackHandoffRecordInput{
		HandoffID: handoffID, TransactionID: correlation.TransactionID, StateSHA256: correlation.StateSHA256,
		AuthorizationCodeSHA256: codeDigest, SealedAuthorizationCode: sealed, RuntimeKeyID: h.config.RuntimeKeyID,
	}); err != nil {
		writeAgentOAuthCallbackError(writer, callbackRecordStatus(err))
		return
	}
	if err = h.config.Notify.Notify(request.Context(), handoffID); err != nil {
		writeAgentOAuthCallbackError(writer, http.StatusServiceUnavailable)
		return
	}
	writer.WriteHeader(http.StatusAccepted)
}

func (h *AgentOAuthCallbackHandler) validate(request *http.Request) (AgentOAuthCallbackCorrelationV1, string, error) {
	var zero AgentOAuthCallbackCorrelationV1
	correlationCookie, err := request.Cookie(h.config.CorrelationCookieName)
	if err != nil || correlationCookie.Value == "" {
		return zero, "", ErrAgentOAuthCallbackHandlerInvalid
	}
	correlation, err := OpenAgentOAuthCallbackCorrelation(correlationCookie.Value, h.config.CorrelationSecret, h.config.Now().UTC())
	if err != nil || correlation.RedirectURI != h.config.RedirectURI {
		return zero, "", ErrAgentOAuthCallbackHandlerInvalid
	}
	browserSession, err := request.Cookie(h.config.BrowserSessionCookieName)
	if err != nil || browserSession.Value == "" || sha256Hex(browserSession.Value) != correlation.BrowserSessionSHA256 {
		return zero, "", ErrAgentOAuthCallbackHandlerInvalid
	}
	query := request.URL.Query()
	state, code := query.Get("state"), query.Get("code")
	if state == "" || sha256Hex(state) != correlation.StateSHA256 || !validAgentOAuthCallbackAuthorizationCode(code) {
		return zero, "", ErrAgentOAuthCallbackHandlerInvalid
	}
	if issuer := query.Get("iss"); issuer != "" && issuer != correlation.Issuer {
		return zero, "", ErrAgentOAuthCallbackHandlerInvalid
	}
	return correlation, code, nil
}

func validAgentOAuthCallbackAuthorizationCode(value string) bool {
	return len(value) > 0 && len(value) <= 4096 && utf8.ValidString(value) && strings.IndexByte(value, 0) < 0
}

func deriveAgentOAuthCallbackHandoffID(secret []byte, correlation AgentOAuthCallbackCorrelationV1, codeSHA256 string) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(strings.Join([]string{
		"dipole.agent.oauth-callback-handoff-id.v1", correlation.TransactionID, correlation.StateSHA256, codeSHA256,
	}, "\n")))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)[:18])
}

func callbackRecordStatus(err error) int {
	if errors.Is(err, ErrAgentOAuthCallbackHandoffRecordInvalid) || errors.Is(err, ErrAgentOAuthCallbackHandoffRecordDenied) {
		return http.StatusBadRequest
	}
	return http.StatusServiceUnavailable
}

func writeAgentOAuthCallbackError(writer http.ResponseWriter, status int) {
	writer.WriteHeader(status)
}
