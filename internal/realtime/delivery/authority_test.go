package delivery

import "testing"

func TestParseAuthority(t *testing.T) {
	for _, test := range []struct {
		value string
		want  Authority
	}{
		{value: "go", want: AuthorityGo},
		{value: " SHADOW ", want: AuthorityShadow},
		{value: "cpp", want: AuthorityCPP},
	} {
		got, err := ParseAuthority(test.value)
		if err != nil {
			t.Fatalf("ParseAuthority(%q): %v", test.value, err)
		}
		if got != test.want {
			t.Fatalf("ParseAuthority(%q) = %q, want %q", test.value, got, test.want)
		}
	}
	if _, err := ParseAuthority(""); err == nil {
		t.Fatal("empty authority must fail closed")
	}
	if _, err := ParseAuthority("dual"); err == nil {
		t.Fatal("unknown authority must fail closed")
	}
}

func TestAuthorityValidatesGatewayCapabilities(t *testing.T) {
	for _, test := range []struct {
		name        string
		authority   Authority
		observation bool
		primary     bool
		wantError   bool
	}{
		{name: "go default", authority: AuthorityGo},
		{name: "go rejects observation", authority: AuthorityGo, observation: true, wantError: true},
		{name: "go rejects primary", authority: AuthorityGo, primary: true, wantError: true},
		{name: "shadow requires observation", authority: AuthorityShadow, wantError: true},
		{name: "shadow observer", authority: AuthorityShadow, observation: true},
		{name: "shadow rejects primary", authority: AuthorityShadow, observation: true, primary: true, wantError: true},
		{name: "cpp requires primary", authority: AuthorityCPP, wantError: true},
		{name: "cpp primary", authority: AuthorityCPP, primary: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := test.authority.ValidateGatewayCapabilities(test.observation, test.primary)
			if (err != nil) != test.wantError {
				t.Fatalf("ValidateGatewayCapabilities() error = %v, wantError = %v", err, test.wantError)
			}
		})
	}
}
