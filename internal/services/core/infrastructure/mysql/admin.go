package coremysql

import (
	"context"
	"errors"
	"fmt"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
	"github.com/JekYUlll/Dipole/internal/model"
)

var _ application.AdminOverviewStore = (*AdminRepository)(nil)

type AdminRepository struct {
	queries generated.Querier
}

func NewAdminRepository(queries generated.Querier) (*AdminRepository, error) {
	if queries == nil {
		return nil, errors.New("admin queries are required")
	}
	return &AdminRepository{queries: queries}, nil
}

func (r *AdminRepository) OverviewCounts() (*application.AdminOverviewCounts, error) {
	row, err := r.queries.GetAdminOverviewCounts(context.Background(), generated.GetAdminOverviewCountsParams{
		DisabledUserStatus:       model.UserStatusDisabled,
		DismissedGroupStatus:     model.GroupStatusDismissed,
		PendingApplicationStatus: model.ContactApplicationPending,
	})
	if err != nil {
		return nil, fmt.Errorf("get admin overview counts with sqlc: %w", err)
	}
	return &application.AdminOverviewCounts{
		UserTotal:                      row.UserTotal,
		AdminUserTotal:                 row.AdminUserTotal,
		DisabledUserTotal:              row.DisabledUserTotal,
		GroupTotal:                     row.GroupTotal,
		DismissedGroupTotal:            row.DismissedGroupTotal,
		MessageTotal:                   row.MessageTotal,
		ConversationTotal:              row.ConversationTotal,
		ContactTotal:                   row.ContactTotal,
		PendingContactApplicationTotal: row.PendingContactApplicationTotal,
	}, nil
}
