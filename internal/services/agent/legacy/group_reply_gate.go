package ai

import (
	"strings"
	"sync"
	"time"
)

const defaultGroupReplyFallback = "抱歉，我这边暂时没能完成回复，请稍后再试。"

type groupReplyGate struct {
	mu         sync.Mutex
	ratePerMin int
	inflight   map[string]struct{}
	stamps     map[string][]time.Time
	now        func() time.Time
}

func newGroupReplyGate(ratePerMin int) *groupReplyGate {
	return &groupReplyGate{
		ratePerMin: ratePerMin,
		inflight:   make(map[string]struct{}),
		stamps:     make(map[string][]time.Time),
		now:        time.Now,
	}
}

func groupReplyAllowed(allowlist []string, groupUUID string) bool {
	groupUUID = strings.TrimSpace(groupUUID)
	if groupUUID == "" {
		return false
	}
	trimmed := make([]string, 0, len(allowlist))
	for _, item := range allowlist {
		item = strings.TrimSpace(item)
		if item != "" {
			trimmed = append(trimmed, item)
		}
	}
	if len(trimmed) == 0 {
		return true
	}
	for _, item := range trimmed {
		if item == groupUUID {
			return true
		}
	}
	return false
}

func resolveGroupReplyFallback(configured string) string {
	if text := strings.TrimSpace(configured); text != "" {
		return text
	}
	return defaultGroupReplyFallback
}

func (g *groupReplyGate) TryEnter(groupUUID string) (release func(), ok bool) {
	if g == nil {
		return func() {}, true
	}
	key := strings.TrimSpace(groupUUID)
	if key == "" {
		return func() {}, false
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if _, busy := g.inflight[key]; busy {
		return func() {}, false
	}

	nowFn := g.now
	if nowFn == nil {
		nowFn = time.Now
	}
	now := nowFn()
	if g.ratePerMin > 0 {
		cutoff := now.Add(-time.Minute)
		kept := g.stamps[key][:0]
		for _, stamp := range g.stamps[key] {
			if stamp.After(cutoff) {
				kept = append(kept, stamp)
			}
		}
		g.stamps[key] = kept
		if len(kept) >= g.ratePerMin {
			return func() {}, false
		}
	}

	g.inflight[key] = struct{}{}
	g.stamps[key] = append(g.stamps[key], now)
	var once sync.Once
	return func() {
		once.Do(func() {
			g.mu.Lock()
			delete(g.inflight, key)
			g.mu.Unlock()
		})
	}, true
}
