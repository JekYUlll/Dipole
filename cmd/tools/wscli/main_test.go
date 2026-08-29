package main

import "testing"

func TestBuildWebSocketURL(t *testing.T) {
	t.Parallel()

	wsURL, err := buildWebSocketURL("http://127.0.0.1:8080", "TOKEN")
	if err != nil {
		t.Fatalf("build websocket url: %v", err)
	}

	expected := "ws://127.0.0.1:8080/api/v1/ws?token=TOKEN"
	if wsURL != expected {
		t.Fatalf("expected %s, got %s", expected, wsURL)
	}
}

func TestBuildHTTPURL(t *testing.T) {
	t.Parallel()

	httpURL, err := buildHTTPURL("127.0.0.1:8080", "/api/v1/auth/login")
	if err != nil {
		t.Fatalf("build http url: %v", err)
	}

	expected := "http://127.0.0.1:8080/api/v1/auth/login"
	if httpURL != expected {
		t.Fatalf("expected %s, got %s", expected, httpURL)
	}
}
