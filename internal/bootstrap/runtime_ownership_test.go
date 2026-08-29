package bootstrap

import "testing"

func TestCoreOwnsMessagePersistenceOnlyInEmbeddedMode(t *testing.T) {
	tests := []struct {
		name      string
		gateway   string
		transport string
		want      bool
	}{
		{name: "embedded local", gateway: "embedded", transport: "local", want: true},
		{name: "embedded grpc", gateway: "embedded", transport: "grpc", want: false},
		{name: "remote local bootstrap", gateway: "remote", transport: "local", want: false},
		{name: "remote grpc", gateway: "remote", transport: "grpc", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := coreOwnsMessagePersistence(test.gateway, test.transport); got != test.want {
				t.Fatalf("coreOwnsMessagePersistence(%q, %q) = %v, want %v", test.gateway, test.transport, got, test.want)
			}
		})
	}
}
