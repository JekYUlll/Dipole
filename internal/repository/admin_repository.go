package repository

import (
	"fmt"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/store"
	"gorm.io/gorm"
)

var _ application.AdminOverviewStore = (*AdminRepository)(nil)

type AdminRepository struct {
	db *gorm.DB
}

func NewAdminRepository() *AdminRepository {
	return &AdminRepository{}
}

func NewAdminRepositoryWithDB(db *gorm.DB) *AdminRepository {
	return &AdminRepository{db: db}
}

func (r *AdminRepository) OverviewCounts() (*application.AdminOverviewCounts, error) {
	counts := &application.AdminOverviewCounts{}

	if err := r.countModel(&model.User{}, &counts.UserTotal); err != nil {
		return nil, fmt.Errorf("count users: %w", err)
	}
	if err := r.countWhere(&model.User{}, "is_admin = ?", &counts.AdminUserTotal, true); err != nil {
		return nil, fmt.Errorf("count admin users: %w", err)
	}
	if err := r.countWhere(&model.User{}, "status = ?", &counts.DisabledUserTotal, model.UserStatusDisabled); err != nil {
		return nil, fmt.Errorf("count disabled users: %w", err)
	}
	if err := r.countModel(&model.Group{}, &counts.GroupTotal); err != nil {
		return nil, fmt.Errorf("count groups: %w", err)
	}
	if err := r.countWhere(&model.Group{}, "status = ?", &counts.DismissedGroupTotal, model.GroupStatusDismissed); err != nil {
		return nil, fmt.Errorf("count dismissed groups: %w", err)
	}
	if err := r.countModel(&model.Message{}, &counts.MessageTotal); err != nil {
		return nil, fmt.Errorf("count messages: %w", err)
	}
	if err := r.countModel(&model.Conversation{}, &counts.ConversationTotal); err != nil {
		return nil, fmt.Errorf("count conversations: %w", err)
	}
	if err := r.countModel(&model.Contact{}, &counts.ContactTotal); err != nil {
		return nil, fmt.Errorf("count contacts: %w", err)
	}
	if err := r.countWhere(&model.ContactApplication{}, "status = ?", &counts.PendingContactApplicationTotal, model.ContactApplicationPending); err != nil {
		return nil, fmt.Errorf("count pending contact applications: %w", err)
	}

	return counts, nil
}

func (r *AdminRepository) countModel(value any, total *int64) error {
	return r.database().Model(value).Count(total).Error
}

func (r *AdminRepository) countWhere(value any, query string, total *int64, args ...any) error {
	return r.database().Model(value).Where(query, args...).Count(total).Error
}

func (r *AdminRepository) database() *gorm.DB {
	if r != nil && r.db != nil {
		return r.db
	}
	return store.DB
}
