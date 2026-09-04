package gateway

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type oauthCallbackRecordStub struct {
	principal string
	input     AgentOAuthCallbackHandoffRecordInput
	err       error
}

func (s *oauthCallbackRecordStub) Record(_ context.Context, principal string, input AgentOAuthCallbackHandoffRecordInput) (*AgentOAuthCallbackHandoffRecordResult, error) {
	s.principal, s.input = principal, input
	if s.err != nil {
		return nil, s.err
	}
	return &AgentOAuthCallbackHandoffRecordResult{HandoffID: input.HandoffID, ExpiresAt: time.Now().UTC().Add(time.Minute)}, nil
}

type oauthCallbackNotifierStub struct {
	handoffID string
	err       error
}

func (s *oauthCallbackNotifierStub) Notify(_ context.Context, handoffID string) error {
	s.handoffID = handoffID
	return s.err
}

func TestAgentOAuthCallbackHandlerRecordsAndNotifiesValidatedCallback(t *testing.T) {
	secret := []byte(strings.Repeat("s", 32))
	state, browserSession := "state-value", "browser-session"
	now := time.Now().UTC().Truncate(time.Millisecond)
	correlationValue := AgentOAuthCallbackCorrelationV1{
		TransactionID: strings.Repeat("a", 22), OwnerUserID: "U100", Issuer: "https://auth.example.com",
		RedirectURI: "https://dipole.example.com/oauth/callback", StateSHA256: sha256Hex(state),
		BrowserSessionSHA256: sha256Hex(browserSession), ExpiresAt: now.Add(time.Minute),
	}
	correlationCookie, err := SealAgentOAuthCallbackCorrelation(correlationValue, secret)
	if err != nil {
		t.Fatal(err)
	}
	record := &oauthCallbackRecordStub{}
	notifier := &oauthCallbackNotifierStub{}
	handler, err := NewAgentOAuthCallbackHandler(AgentOAuthCallbackHandlerConfig{
		CorrelationSecret: secret, BrowserSessionCookieName: "dipole_browser_session", CorrelationCookieName: "dipole_oauth_correlation",
		RedirectURI: correlationValue.RedirectURI, RuntimePublicKeyPEM: testAgentOAuthCallbackPublicKey(t), RuntimeKeyID: "runtime-key-1",
		Record: record, Notify: notifier, Now: func() time.Time { return now }, DeriveHandoffID: func(AgentOAuthCallbackCorrelationV1, string) (string, error) { return strings.Repeat("b", 22), nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/oauth/callback?code=authorization-code&state="+state+"&iss=https%3A%2F%2Fauth.example.com", nil)
	request.AddCookie(&http.Cookie{Name: "dipole_oauth_correlation", Value: correlationCookie})
	request.AddCookie(&http.Cookie{Name: "dipole_browser_session", Value: browserSession})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted || record.principal != "U100" || record.input.HandoffID != strings.Repeat("b", 22) ||
		record.input.TransactionID != correlationValue.TransactionID || record.input.StateSHA256 != correlationValue.StateSHA256 ||
		record.input.AuthorizationCodeSHA256 != sha256Hex("authorization-code") || notifier.handoffID != record.input.HandoffID {
		t.Fatalf("status=%d principal=%q record=%+v notified=%q", response.Code, record.principal, record.input, notifier.handoffID)
	}
	if response.Header().Get("Cache-Control") != "no-store" || strings.Contains(response.Body.String(), "authorization-code") {
		t.Fatalf("unexpected response: headers=%v body=%q", response.Header(), response.Body.String())
	}
}

func TestAgentOAuthCallbackHandlerFailsClosedBeforeRecordAndAllowsSafeNotificationRetry(t *testing.T) {
	secret := []byte(strings.Repeat("s", 32))
	state, browserSession := "state-value", "browser-session"
	now := time.Now().UTC().Truncate(time.Millisecond)
	correlationValue := AgentOAuthCallbackCorrelationV1{
		TransactionID: strings.Repeat("a", 22), OwnerUserID: "U100", Issuer: "https://auth.example.com",
		RedirectURI: "https://dipole.example.com/oauth/callback", StateSHA256: sha256Hex(state),
		BrowserSessionSHA256: sha256Hex(browserSession), ExpiresAt: now.Add(time.Minute),
	}
	correlationCookie, _ := SealAgentOAuthCallbackCorrelation(correlationValue, secret)
	record := &oauthCallbackRecordStub{}
	notifier := &oauthCallbackNotifierStub{err: ErrAgentOAuthCallbackHandoffNotifierUnavailable}
	handler, err := NewAgentOAuthCallbackHandler(AgentOAuthCallbackHandlerConfig{
		CorrelationSecret: secret, BrowserSessionCookieName: "dipole_browser_session", CorrelationCookieName: "dipole_oauth_correlation",
		RedirectURI: correlationValue.RedirectURI, RuntimePublicKeyPEM: testAgentOAuthCallbackPublicKey(t), RuntimeKeyID: "runtime-key-1",
		Record: record, Notify: notifier, Now: func() time.Time { return now }, DeriveHandoffID: func(AgentOAuthCallbackCorrelationV1, string) (string, error) { return strings.Repeat("b", 22), nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	invalid := httptest.NewRequest(http.MethodGet, "/oauth/callback?code=authorization-code&state=wrong", nil)
	invalid.AddCookie(&http.Cookie{Name: "dipole_oauth_correlation", Value: correlationCookie})
	invalid.AddCookie(&http.Cookie{Name: "dipole_browser_session", Value: browserSession})
	invalidResponse := httptest.NewRecorder()
	handler.ServeHTTP(invalidResponse, invalid)
	if invalidResponse.Code != http.StatusBadRequest || record.principal != "" {
		t.Fatalf("invalid status=%d record=%+v", invalidResponse.Code, record)
	}

	valid := httptest.NewRequest(http.MethodGet, "/oauth/callback?code=authorization-code&state="+state, nil)
	valid.AddCookie(&http.Cookie{Name: "dipole_oauth_correlation", Value: correlationCookie})
	valid.AddCookie(&http.Cookie{Name: "dipole_browser_session", Value: browserSession})
	validResponse := httptest.NewRecorder()
	handler.ServeHTTP(validResponse, valid)
	if validResponse.Code != http.StatusServiceUnavailable || record.principal != "U100" || notifier.handoffID == "" {
		t.Fatalf("retry status=%d record=%+v notified=%q", validResponse.Code, record, notifier.handoffID)
	}
}

func TestAgentOAuthCallbackHandlerDerivesStableHandoffIDPerCallbackBinding(t *testing.T) {
	correlation := AgentOAuthCallbackCorrelationV1{TransactionID: strings.Repeat("a", 22), StateSHA256: strings.Repeat("b", 64)}
	first := deriveAgentOAuthCallbackHandoffID([]byte(strings.Repeat("s", 32)), correlation, strings.Repeat("c", 64))
	if first != deriveAgentOAuthCallbackHandoffID([]byte(strings.Repeat("s", 32)), correlation, strings.Repeat("c", 64)) || !validOAuthTransactionID(first) {
		t.Fatalf("handoff ID is not stable and valid: %q", first)
	}
	if first == deriveAgentOAuthCallbackHandoffID([]byte(strings.Repeat("s", 32)), correlation, strings.Repeat("d", 64)) {
		t.Fatal("authorization code digest must affect the handoff ID")
	}
}

func testAgentOAuthCallbackPublicKey(t *testing.T) []byte {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: encoded})
}
