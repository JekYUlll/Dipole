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
	mountWebApp(engine, nil)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/app/", nil)

	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "<title>Dipole</title>") {
		t.Fatalf("expected web app html to be served")
	}
}

func TestWebAppIndexInjectsFrontendFlags(t *testing.T) {
	t.Parallel()

	engine := gin.New()
	mountWebApp(engine, &FrontendFlags{TaskCreate: true, Timeline: true, Memories: true})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/app/", nil)

	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "window.__DIPOLE_FLAGS__=") {
		t.Fatalf("expected injected flags script in html")
	}
	if !strings.Contains(body, `"taskCreate":true`) {
		t.Fatalf("expected taskCreate flag in injected script")
	}
	if !strings.Contains(body, `"memories":true`) {
		t.Fatalf("expected memories flag in injected script")
	}
	if !strings.Contains(body, "</head>") {
		t.Fatalf("expected closing head tag preserved")
	}
}

func TestWebAppIndexWithNilFlagsHasNoScript(t *testing.T) {
	t.Parallel()

	engine := gin.New()
	mountWebApp(engine, nil)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/app/", nil)

	engine.ServeHTTP(recorder, request)

	body := recorder.Body.String()
	if strings.Contains(body, "__DIPOLE_FLAGS__") {
		t.Fatalf("expected no flags script when flags are nil")
	}
}

func TestRootRedirectsToWebApp(t *testing.T) {
	t.Parallel()

	engine := gin.New()
	mountWebApp(engine, nil)
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
