package httpdto

import "github.com/JekYUlll/Dipole/internal/service"

type AdminOverviewResponse struct {
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

func ToAdminOverviewResponse(overview *service.AdminOverview) *AdminOverviewResponse {
	if overview == nil {
		return nil
	}

	return &AdminOverviewResponse{
		AppName:                        overview.AppName,
		Env:                            overview.Env,
		UserTotal:                      overview.UserTotal,
		AdminUserTotal:                 overview.AdminUserTotal,
		DisabledUserTotal:              overview.DisabledUserTotal,
		GroupTotal:                     overview.GroupTotal,
		DismissedGroupTotal:            overview.DismissedGroupTotal,
		MessageTotal:                   overview.MessageTotal,
		ConversationTotal:              overview.ConversationTotal,
		ContactTotal:                   overview.ContactTotal,
		PendingContactApplicationTotal: overview.PendingContactApplicationTotal,
		OnlineUserTotal:                overview.OnlineUserTotal,
		OnlineConnectionTotal:          overview.OnlineConnectionTotal,
		KafkaEnabled:                   overview.KafkaEnabled,
		TLSEnabled:                     overview.TLSEnabled,
	}
}
