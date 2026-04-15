package repository

import (
	"fmt"

	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/store"
)

type AdminOverviewCounts struct {
	UserTotal                      int64
	AdminUserTotal                 int64
	DisabledUserTotal              int64
	GroupTotal                     int64
	DismissedGroupTotal            int64
	MessageTotal                   int64
	ConversationTotal              int64
	ContactTotal                   int64
	PendingContactApplicationTotal int64
}

type AdminRepository struct{}

func NewAdminRepository() *AdminRepository {
	return &AdminRepository{}
}

func (r *AdminRepository) OverviewCounts() (*AdminOverviewCounts, error) {
	counts := &AdminOverviewCounts{}

	if err := countModel(&model.User{}, &counts.UserTotal); err != nil {
		return nil, fmt.Errorf("count users: %w", err)
	}
	if err := countWhere(&model.User{}, "is_admin = ?", &counts.AdminUserTotal, true); err != nil {
		return nil, fmt.Errorf("count admin users: %w", err)
	}
	if err := countWhere(&model.User{}, "status = ?", &counts.DisabledUserTotal, model.UserStatusDisabled); err != nil {
		return nil, fmt.Errorf("count disabled users: %w", err)
	}
	if err := countModel(&model.Group{}, &counts.GroupTotal); err != nil {
		return nil, fmt.Errorf("count groups: %w", err)
	}
	if err := countWhere(&model.Group{}, "status = ?", &counts.DismissedGroupTotal, model.GroupStatusDismissed); err != nil {
		return nil, fmt.Errorf("count dismissed groups: %w", err)
	}
	if err := countModel(&model.Message{}, &counts.MessageTotal); err != nil {
		return nil, fmt.Errorf("count messages: %w", err)
	}
	if err := countModel(&model.Conversation{}, &counts.ConversationTotal); err != nil {
		return nil, fmt.Errorf("count conversations: %w", err)
	}
	if err := countModel(&model.Contact{}, &counts.ContactTotal); err != nil {
		return nil, fmt.Errorf("count contacts: %w", err)
	}
	if err := countWhere(&model.ContactApplication{}, "status = ?", &counts.PendingContactApplicationTotal, model.ContactApplicationPending); err != nil {
		return nil, fmt.Errorf("count pending contact applications: %w", err)
	}

	return counts, nil
}

func countModel(value any, total *int64) error {
	return store.DB.Model(value).Count(total).Error
}

func countWhere(value any, query string, total *int64, args ...any) error {
	return store.DB.Model(value).Where(query, args...).Count(total).Error
}
