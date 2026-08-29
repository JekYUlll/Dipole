package gateway

import "testing"

func TestNewServerWithDependenciesRequiresTokenResolver(t *testing.T) {
	_, err := NewServerWithDependencies("http://127.0.0.1:8081", Dependencies{
		Messages: gatewayMessageStub{},
		Core:     gatewayCoreStub{},
	})
	if err == nil {
		t.Fatal("explicit Gateway composition must require a token resolver")
	}
}
