package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type presignedUploadLimiterStub struct {
	identifier string
	allowed    bool
	retryAfter time.Duration
}

func (s *presignedUploadLimiterStub) AllowFileUpload(identifier string) (bool, time.Duration) {
	s.identifier = identifier
	return s.allowed, s.retryAfter
}

func TestPresignedUploadHandlerRateLimitsBeforeProxy(t *testing.T) {
	limiter := &presignedUploadLimiterStub{retryAfter: 1500 * time.Millisecond}
	proxyCalls := 0
	proxy := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { proxyCalls++ })
	handler := presignedUploadHandler(proxy, limiter)

	request := httptest.NewRequest(http.MethodPut, "/dipole-files/object", nil)
	request.RemoteAddr = "198.51.100.10:4321"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusTooManyRequests || recorder.Header().Get("Retry-After") != "2" {
		t.Fatalf("unexpected rate-limit response: code=%d retry=%q", recorder.Code, recorder.Header().Get("Retry-After"))
	}
	if limiter.identifier != "198.51.100.10" || proxyCalls != 0 {
		t.Fatalf("rate limit did not use client address or leaked to proxy: identifier=%q calls=%d", limiter.identifier, proxyCalls)
	}
}

func TestPresignedUploadHandlerForwardsWhenAllowed(t *testing.T) {
	limiter := &presignedUploadLimiterStub{allowed: true}
	proxyCalls := 0
	handler := presignedUploadHandler(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		proxyCalls++
		writer.WriteHeader(http.StatusOK)
	}), limiter)

	request := httptest.NewRequest(http.MethodPut, "/dipole-files/object", nil)
	request.RemoteAddr = "203.0.113.7:9876"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK || proxyCalls != 1 || limiter.identifier != "203.0.113.7" {
		t.Fatalf("allowed request was not forwarded: code=%d calls=%d identifier=%q", recorder.Code, proxyCalls, limiter.identifier)
	}
}
