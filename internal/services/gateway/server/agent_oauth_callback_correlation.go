package gateway

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var ErrAgentOAuthCallbackCorrelationInvalid = errors.New("Agent OAuth callback correlation is invalid")

// AgentOAuthCallbackCorrelationV1 is the verified browser binding required
// before a future callback handler may record a durable handoff.
type AgentOAuthCallbackCorrelationV1 struct {
	TransactionID, OwnerUserID, Issuer, RedirectURI string
	StateSHA256, BrowserSessionSHA256               string
	ExpiresAt                                       time.Time
}

func SealAgentOAuthCallbackCorrelation(v AgentOAuthCallbackCorrelationV1, secret []byte) (string, error) {
	if !validAgentOAuthCallbackCorrelation(v, time.Time{}) || len(secret) < 32 {
		return "", ErrAgentOAuthCallbackCorrelationInvalid
	}
	payload, err := json.Marshal(struct {
		TransactionID, OwnerUserID, Issuer, RedirectURI, StateSHA256, BrowserSessionSHA256 string
		ExpiresAtUnixMs                                                                    int64
	}{v.TransactionID, v.OwnerUserID, v.Issuer, v.RedirectURI, v.StateSHA256, v.BrowserSessionSHA256, v.ExpiresAt.UTC().UnixMilli()})
	if err != nil {
		return "", ErrAgentOAuthCallbackCorrelationInvalid
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(encoded))
	return "v1." + encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func OpenAgentOAuthCallbackCorrelation(token string, secret []byte, now time.Time) (AgentOAuthCallbackCorrelationV1, error) {
	var zero AgentOAuthCallbackCorrelationV1
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != "v1" || len(secret) < 32 || now.IsZero() {
		return zero, ErrAgentOAuthCallbackCorrelationInvalid
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(parts[1]))
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(signature, mac.Sum(nil)) {
		return zero, ErrAgentOAuthCallbackCorrelationInvalid
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return zero, ErrAgentOAuthCallbackCorrelationInvalid
	}
	var wire struct {
		TransactionID, OwnerUserID, Issuer, RedirectURI, StateSHA256, BrowserSessionSHA256 string
		ExpiresAtUnixMs                                                                    int64
	}
	if json.Unmarshal(payload, &wire) != nil {
		return zero, ErrAgentOAuthCallbackCorrelationInvalid
	}
	v := AgentOAuthCallbackCorrelationV1{TransactionID: wire.TransactionID, OwnerUserID: wire.OwnerUserID, Issuer: wire.Issuer, RedirectURI: wire.RedirectURI, StateSHA256: wire.StateSHA256, BrowserSessionSHA256: wire.BrowserSessionSHA256, ExpiresAt: time.UnixMilli(wire.ExpiresAtUnixMs).UTC()}
	if !validAgentOAuthCallbackCorrelation(v, now) {
		return zero, ErrAgentOAuthCallbackCorrelationInvalid
	}
	return v, nil
}

func validAgentOAuthCallbackCorrelation(v AgentOAuthCallbackCorrelationV1, now time.Time) bool {
	return validOAuthTransactionID(v.TransactionID) && validAgentSubscriptionPublicID(v.OwnerUserID, 64) && validOAuthURL(v.Issuer) && validOAuthURL(v.RedirectURI) && isLowerHex(v.StateSHA256) && len(v.StateSHA256) == 64 && isLowerHex(v.BrowserSessionSHA256) && len(v.BrowserSessionSHA256) == 64 && !v.ExpiresAt.IsZero() && (now.IsZero() || v.ExpiresAt.After(now))
}
