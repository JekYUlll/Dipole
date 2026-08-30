package gateway

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/platform/correlation"
)

func TestAgentOAuthCallbackHandoffNotifierSendsOnlyHandoffAndCorrelation(t *testing.T) {
	var gotBody []byte
	var gotHeader http.Header
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != agentOAuthCallbackHandoffNotifyPath {
			t.Fatalf("method=%s path=%s", request.Method, request.URL.Path)
		}
		gotBody, _ = io.ReadAll(request.Body)
		gotHeader = request.Header.Clone()
		writer.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	notifier, err := NewAgentOAuthCallbackHandoffNotifier(server.URL, "gateway-secret", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	ctx := correlation.WithContext(context.Background(), correlation.IDs{RequestID: "REQ-1", TraceID: "TRACE-1"})
	if err := notifier.Notify(ctx, "aaaaaaaaaaaaaaaaaaaaaa"); err != nil {
		t.Fatal(err)
	}
	if string(gotBody) != `{"handoff_id":"aaaaaaaaaaaaaaaaaaaaaa"}` {
		t.Fatalf("unexpected body: %s", gotBody)
	}
	if gotHeader.Get("X-Dipole-Caller-Service") != "dipole-gateway" || gotHeader.Get("X-Dipole-Service-Token") != "gateway-secret" ||
		gotHeader.Get(correlation.RequestHeader) != "REQ-1" || gotHeader.Get(correlation.TraceHeader) != "TRACE-1" || gotHeader.Get("X-Dipole-Principal-User-ID") != "" {
		t.Fatalf("unexpected headers: %+v", gotHeader)
	}
}

func TestAgentOAuthCallbackHandoffNotifierFailsClosed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusInternalServerError)
		_, _ = writer.Write([]byte("do not expose response"))
	}))
	defer server.Close()
	cases := []struct {
		name    string
		target  string
		secret  string
		handoff string
		want    error
	}{
		{name: "remote plaintext", target: "http://runtime.example.com", secret: "secret", handoff: "aaaaaaaaaaaaaaaaaaaaaa", want: ErrAgentOAuthCallbackHandoffNotifierInvalid},
		{name: "missing secret", target: server.URL, secret: "", handoff: "aaaaaaaaaaaaaaaaaaaaaa", want: ErrAgentOAuthCallbackHandoffNotifierInvalid},
		{name: "invalid handoff", target: server.URL, secret: "secret", handoff: "handoff with whitespace", want: ErrAgentOAuthCallbackHandoffNotifierInvalid},
		{name: "runtime rejected", target: server.URL, secret: "secret", handoff: "aaaaaaaaaaaaaaaaaaaaaa", want: ErrAgentOAuthCallbackHandoffNotifierUnavailable},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			notifier, err := NewAgentOAuthCallbackHandoffNotifier(testCase.target, testCase.secret, time.Second)
			if err == nil {
				err = notifier.Notify(context.Background(), testCase.handoff)
			}
			if !errors.Is(err, testCase.want) {
				t.Fatalf("error=%v want=%v", err, testCase.want)
			}
		})
	}
}
