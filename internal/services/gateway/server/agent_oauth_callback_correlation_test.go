package gateway

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestAgentOAuthCallbackCorrelationBindsStateAndBrowserSession(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	value := AgentOAuthCallbackCorrelationV1{TransactionID: strings.Repeat("a", 22), OwnerUserID: "U100", Issuer: "https://auth.example.com", RedirectURI: "https://dipole.example.com/oauth/callback", StateSHA256: strings.Repeat("b", 64), BrowserSessionSHA256: strings.Repeat("c", 64), ExpiresAt: now.Add(time.Minute)}
	token, err := SealAgentOAuthCallbackCorrelation(value, []byte(strings.Repeat("s", 32)))
	if err != nil {
		t.Fatal(err)
	}
	opened, err := OpenAgentOAuthCallbackCorrelation(token, []byte(strings.Repeat("s", 32)), now)
	if err != nil || opened != value {
		t.Fatalf("opened=%+v err=%v", opened, err)
	}
	if _, err = OpenAgentOAuthCallbackCorrelation(token+"x", []byte(strings.Repeat("s", 32)), now); !errors.Is(err, ErrAgentOAuthCallbackCorrelationInvalid) {
		t.Fatalf("error=%v", err)
	}
	if _, err = OpenAgentOAuthCallbackCorrelation(token, []byte(strings.Repeat("s", 32)), value.ExpiresAt); !errors.Is(err, ErrAgentOAuthCallbackCorrelationInvalid) {
		t.Fatalf("error=%v", err)
	}
}
