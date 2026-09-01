package delivery

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"strings"
)

// GrayScope identifies the stable identity used for a rollout decision.
type GrayScope string

const (
	GrayScopeNode GrayScope = "node"
	GrayScopeUser GrayScope = "user"
)

// RolloutPolicy deterministically assigns a subject to Go or the target
// authority. It is intentionally independent from the delivery hot path.
type RolloutPolicy struct {
	Scope      GrayScope
	Target     Authority
	Percentage uint8
	Salt       string
}

func (p RolloutPolicy) Validate() error {
	if p.Scope != GrayScopeNode && p.Scope != GrayScopeUser {
		return fmt.Errorf("unsupported realtime rollout scope %q", p.Scope)
	}
	if p.Target != AuthorityShadow && p.Target != AuthorityCPP {
		return fmt.Errorf("realtime rollout target must be shadow or cpp, got %q", p.Target)
	}
	if p.Percentage > 100 {
		return fmt.Errorf("realtime rollout percentage must be between 0 and 100, got %d", p.Percentage)
	}
	if p.Salt == "" {
		return fmt.Errorf("realtime rollout salt is required")
	}
	return nil
}

// Select returns the authority for a subject. A malformed subject is rejected
// so callers cannot accidentally turn an unscoped rollout into a broad one.
func (p RolloutPolicy) Select(subject string) (Authority, error) {
	if err := p.Validate(); err != nil {
		return "", err
	}
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return "", fmt.Errorf("realtime rollout subject is required")
	}
	if p.Percentage == 0 {
		return AuthorityGo, nil
	}
	if p.Percentage == 100 {
		return p.Target, nil
	}

	digest := sha256.Sum256([]byte(string(p.Scope) + "\x00" + p.Salt + "\x00" + subject))
	bucket := binary.BigEndian.Uint64(digest[:8]) % 100
	if bucket < uint64(p.Percentage) {
		return p.Target, nil
	}
	return AuthorityGo, nil
}
