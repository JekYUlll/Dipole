package hotgroup

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/store"
)

func TestRedisDetectorDoesNotMarkSmallGroupAsHot(t *testing.T) {
	cleanup := setupRedisDetectorTest(t)
	defer cleanup()

	detector := NewDetector(config.HotGroup{
		Enabled:              true,
		MemberCountThreshold: 5,
		MessageThreshold:     3,
		WindowSeconds:        60,
		CoolingSeconds:       180,
	})

	for range 5 {
		status, err := detector.ObserveMessage("G100", 4)
		if err != nil {
			t.Fatalf("observe message: %v", err)
		}
		if status.IsHot {
			t.Fatalf("expected non-hot small group, got %+v", status)
		}
	}
}

func TestRedisDetectorMarksHotGroupAfterThreshold(t *testing.T) {
	cleanup := setupRedisDetectorTest(t)
	defer cleanup()

	detector := NewDetector(config.HotGroup{
		Enabled:              true,
		MemberCountThreshold: 5,
		MessageThreshold:     3,
		WindowSeconds:        60,
		CoolingSeconds:       180,
	})

	for i := 0; i < 2; i++ {
		status, err := detector.ObserveMessage("G100", 8)
		if err != nil {
			t.Fatalf("observe message: %v", err)
		}
		if status.IsHot {
			t.Fatalf("expected cold group before threshold, got %+v", status)
		}
	}

	status, err := detector.ObserveMessage("G100", 8)
	if err != nil {
		t.Fatalf("observe message at threshold: %v", err)
	}
	if !status.IsHot {
		t.Fatalf("expected hot group after threshold, got %+v", status)
	}
	if status.RecentMessageCount != 3 {
		t.Fatalf("expected recent message count 3, got %d", status.RecentMessageCount)
	}

	check, err := detector.Status("G100", 8)
	if err != nil {
		t.Fatalf("query status: %v", err)
	}
	if !check.IsHot {
		t.Fatalf("expected hot group status, got %+v", check)
	}
}

func setupRedisDetectorTest(t *testing.T) func() {
	t.Helper()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("run miniredis: %v", err)
	}

	originalRDB := store.RDB
	store.RDB = redis.NewClient(&redis.Options{Addr: mr.Addr()})

	return func() {
		if store.RDB != nil {
			_ = store.RDB.Close()
		}
		store.RDB = originalRDB
		mr.Close()
	}
}
