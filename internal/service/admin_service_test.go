package service

import (
	"errors"
	"testing"

	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/repository"
)

type stubAdminOverviewRepository struct {
	counts *repository.AdminOverviewCounts
	err    error
}

func (r *stubAdminOverviewRepository) OverviewCounts() (*repository.AdminOverviewCounts, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.counts, nil
}

type stubAdminRealtimeStats struct {
	onlineUsers       int
	onlineConnections int
}

func (s *stubAdminRealtimeStats) OnlineUserCount() int {
	return s.onlineUsers
}

func (s *stubAdminRealtimeStats) TotalConnectionCount() int {
	return s.onlineConnections
}

func TestAdminServiceOverviewSuccess(t *testing.T) {
	t.Parallel()

	service := newAdminService(&stubAdminOverviewRepository{
		counts: &repository.AdminOverviewCounts{
			UserTotal:                      10,
			AdminUserTotal:                 1,
			DisabledUserTotal:              2,
			GroupTotal:                     3,
			DismissedGroupTotal:            1,
			MessageTotal:                   88,
			ConversationTotal:              16,
			ContactTotal:                   12,
			PendingContactApplicationTotal: 4,
		},
	}, &stubAdminRealtimeStats{
		onlineUsers:       5,
		onlineConnections: 7,
	}, adminRuntimeSnapshot{
		appName:      "dipole",
		env:          "test",
		kafkaEnabled: true,
		tlsEnabled:   true,
	})

	overview, err := service.Overview(&model.User{UUID: "U1", IsAdmin: true})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if overview.UserTotal != 10 || overview.MessageTotal != 88 {
		t.Fatalf("unexpected overview counts: %+v", overview)
	}
	if overview.OnlineUserTotal != 5 || overview.OnlineConnectionTotal != 7 {
		t.Fatalf("unexpected realtime overview: %+v", overview)
	}
}

func TestAdminServiceOverviewRequiresAdmin(t *testing.T) {
	t.Parallel()

	service := newAdminService(&stubAdminOverviewRepository{}, &stubAdminRealtimeStats{}, adminRuntimeSnapshot{})

	_, err := service.Overview(&model.User{UUID: "U1", IsAdmin: false})
	if !errors.Is(err, ErrAdminRequired) {
		t.Fatalf("expected ErrAdminRequired, got %v", err)
	}
}
