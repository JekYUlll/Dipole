package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestWebAppIndexServed(t *testing.T) {
	t.Parallel()

	engine := gin.New()
	mountWebApp(engine)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/app/", nil)

	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "<title>Dipole Web Tester</title>") {
		t.Fatalf("expected web app html to be served")
	}
}

func TestRootRedirectsToWebApp(t *testing.T) {
	t.Parallel()

	engine := gin.New()
	mountWebApp(engine)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected status 307, got %d", recorder.Code)
	}
	if location := recorder.Header().Get("Location"); location != "/app/" {
		t.Fatalf("expected redirect to /app/, got %q", location)
	}
}
