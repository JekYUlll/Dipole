package server

import "testing"

func TestCoreOwnsHTTPDataRoutesOnlyInEmbeddedMode(t *testing.T) {
	for _, test := range []struct {
		mode string
		want bool
	}{
		{mode: "embedded", want: true},
		{mode: "remote", want: false},
		{mode: "", want: false},
	} {
		if got := coreOwnsHTTPDataRoutes(test.mode); got != test.want {
			t.Fatalf("coreOwnsHTTPDataRoutes(%q) = %v, want %v", test.mode, got, test.want)
		}
	}
}
