package service

import (
	"fmt"

	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/repository"
)

type adminOverviewRepository interface {
	OverviewCounts() (*repository.AdminOverviewCounts, error)
}

type adminRealtimeStats interface {
	OnlineUserCount() int
	TotalConnectionCount() int
}

type AdminOverview struct {
	AppName                        string `json:"app_name"`
	Env                            string `json:"env"`
	UserTotal                      int64  `json:"user_total"`
	AdminUserTotal                 int64  `json:"admin_user_total"`
	DisabledUserTotal              int64  `json:"disabled_user_total"`
	GroupTotal                     int64  `json:"group_total"`
	DismissedGroupTotal            int64  `json:"dismissed_group_total"`
	MessageTotal                   int64  `json:"message_total"`
	ConversationTotal              int64  `json:"conversation_total"`
	ContactTotal                   int64  `json:"contact_total"`
	PendingContactApplicationTotal int64  `json:"pending_contact_application_total"`
	OnlineUserTotal                int    `json:"online_user_total"`
	OnlineConnectionTotal          int    `json:"online_connection_total"`
	KafkaEnabled                   bool   `json:"kafka_enabled"`
	TLSEnabled                     bool   `json:"tls_enabled"`
}

type AdminService struct {
	repo     adminOverviewRepository
	realtime adminRealtimeStats
	runtime  adminRuntimeSnapshot
}

func NewAdminService(repo adminOverviewRepository, realtime adminRealtimeStats) *AdminService {
	appCfg := config.AppConfig()
	tlsCfg := config.TLSConfig()
	kafkaCfg := config.KafkaConfig()

	return newAdminService(repo, realtime, adminRuntimeSnapshot{
		appName:      appCfg.Name,
		env:          appCfg.Env,
		kafkaEnabled: kafkaCfg.Enabled,
		tlsEnabled:   tlsCfg.Enabled,
	})
}

type adminRuntimeSnapshot struct {
	appName      string
	env          string
	kafkaEnabled bool
	tlsEnabled   bool
}

func newAdminService(repo adminOverviewRepository, realtime adminRealtimeStats, runtime adminRuntimeSnapshot) *AdminService {
	return &AdminService{
		repo:     repo,
		realtime: realtime,
		runtime:  runtime,
	}
}

func (s *AdminService) Overview(currentUser *model.User) (*AdminOverview, error) {
	if currentUser == nil || !currentUser.IsAdmin {
		return nil, ErrAdminRequired
	}

	counts, err := s.repo.OverviewCounts()
	if err != nil {
		return nil, fmt.Errorf("load admin overview counts: %w", err)
	}

	overview := &AdminOverview{
		AppName:                        s.runtime.appName,
		Env:                            s.runtime.env,
		UserTotal:                      counts.UserTotal,
		AdminUserTotal:                 counts.AdminUserTotal,
		DisabledUserTotal:              counts.DisabledUserTotal,
		GroupTotal:                     counts.GroupTotal,
		DismissedGroupTotal:            counts.DismissedGroupTotal,
		MessageTotal:                   counts.MessageTotal,
		ConversationTotal:              counts.ConversationTotal,
		ContactTotal:                   counts.ContactTotal,
		PendingContactApplicationTotal: counts.PendingContactApplicationTotal,
		KafkaEnabled:                   s.runtime.kafkaEnabled,
		TLSEnabled:                     s.runtime.tlsEnabled,
	}
	if s.realtime != nil {
		overview.OnlineUserTotal = s.realtime.OnlineUserCount()
		overview.OnlineConnectionTotal = s.realtime.TotalConnectionCount()
	}

	return overview, nil
}
