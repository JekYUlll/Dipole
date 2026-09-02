package gateway

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPresignedUploadProxyForwardsSignedPutAndPreservesSignedHost(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Host != "storage.example.test" {
			t.Fatalf("unexpected signed host: %q", request.Host)
		}
		body, _ := io.ReadAll(request.Body)
		if string(body) != "hello" {
			t.Fatalf("unexpected body: %q", body)
		}
		writer.Header().Set("ETag", `"etag-1"`)
		writer.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	proxy, err := NewPresignedUploadProxy(upstream.URL, "storage.example.test", 5)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/dipole-files/message-files/a?partNumber=1&uploadId=up-1&X-Amz-Credential=key&X-Amz-Signature=sig", strings.NewReader("hello"))
	proxy.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Header().Get("ETag") != `"etag-1"` {
		t.Fatalf("unexpected response: code=%d headers=%v", recorder.Code, recorder.Header())
	}
}

func TestPresignedUploadProxyRejectsUnsignedOrOversizedRequest(t *testing.T) {
	proxy, err := NewPresignedUploadProxy("http://127.0.0.1:1", "storage.example.test", 4)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		url  string
		body string
	}{
		{name: "unsigned", url: "/dipole-files/a", body: "ok"},
		{name: "oversized", url: "/dipole-files/a?partNumber=1&uploadId=u&X-Amz-Credential=k&X-Amz-Signature=s", body: "large"},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPut, test.url, strings.NewReader(test.body))
			proxy.ServeHTTP(recorder, request)
			expected := http.StatusForbidden
			if test.name == "oversized" {
				expected = http.StatusRequestEntityTooLarge
			}
			if recorder.Code != expected {
				t.Fatalf("unexpected status: got=%d want=%d", recorder.Code, expected)
			}
		})
	}
}

func TestPresignedUploadProxyReturnsBadGatewayWhenUpstreamTimesOut(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		time.Sleep(50 * time.Millisecond)
	}))
	defer upstream.Close()

	proxy, err := NewPresignedUploadProxyWithTimeout(upstream.URL, "storage.example.test", 5, 5*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/dipole-files/message-files/a?partNumber=1&uploadId=up-1&X-Amz-Credential=key&X-Amz-Signature=sig", strings.NewReader("hello"))
	proxy.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502 after upstream timeout, got %d", recorder.Code)
	}
}

func TestPresignedUploadProxyRejectsNonPositiveTimeout(t *testing.T) {
	if _, err := NewPresignedUploadProxyWithTimeout("http://127.0.0.1:9000", "storage.example.test", 5, 0); err == nil {
		t.Fatal("non-positive upstream timeout was accepted")
	}
}
