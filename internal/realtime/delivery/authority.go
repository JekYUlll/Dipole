package delivery

import (
	"fmt"
	"strings"
)

type Authority string

const (
	AuthorityGo     Authority = "go"
	AuthorityShadow Authority = "shadow"
	AuthorityCPP    Authority = "cpp"
)

func ParseAuthority(value string) (Authority, error) {
	authority := Authority(strings.ToLower(strings.TrimSpace(value)))
	switch authority {
	case AuthorityGo, AuthorityShadow, AuthorityCPP:
		return authority, nil
	default:
		return "", fmt.Errorf("unsupported realtime delivery authority %q", value)
	}
}

func (a Authority) ValidateGatewayCapabilities(observationEnabled, primaryEnabled bool) error {
	switch a {
	case AuthorityGo:
		if observationEnabled || primaryEnabled {
			return fmt.Errorf("realtime delivery authority %q rejects C++ delivery capabilities", a)
		}
	case AuthorityShadow:
		if !observationEnabled || primaryEnabled {
			return fmt.Errorf("realtime delivery authority %q requires observation-only delivery", a)
		}
	case AuthorityCPP:
		if !primaryEnabled {
			return fmt.Errorf("realtime delivery authority %q requires primary delivery", a)
		}
	default:
		return fmt.Errorf("unsupported realtime delivery authority %q", a)
	}
	return nil
}
