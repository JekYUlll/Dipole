package repository

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/store"
)

var _ application.GroupStore = (*GroupRepository)(nil)

type GroupRepository struct {
	db *gorm.DB
}

func NewGroupRepository() *GroupRepository {
	return &GroupRepository{}
}

func NewGroupRepositoryWithDB(db *gorm.DB) *GroupRepository {
	return &GroupRepository{db: db}
}

func (r *GroupRepository) Create(group *model.Group, members []*model.GroupMember) error {
	return r.database().Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(group).Error; err != nil {
			return fmt.Errorf("create group: %w", err)
		}
		valid := nonNilGroupMembers(members)
		if len(valid) == 0 {
			return nil
		}
		rows := make([]map[string]any, 0, len(valid))
		for _, member := range valid {
			rows = append(rows, map[string]any{
				"group_uuid": member.GroupUUID,
				"user_uuid":  member.UserUUID,
				"role":       member.Role,
				"joined_at":  member.JoinedAt,
				"created_at": member.CreatedAt,
				"updated_at": member.UpdatedAt,
			})
		}
		if err := tx.Table(model.GroupMember{}.TableName()).Create(rows).Error; err != nil {
			return fmt.Errorf("create group members: %w", err)
		}
		return nil
	})
}

func (r *GroupRepository) GetByUUID(groupUUID string) (*model.Group, error) {
	groupUUID = strings.TrimSpace(groupUUID)
	if groupUUID == "" {
		return nil, nil
	}
	var group model.Group
	if err := r.database().Where("uuid = ?", groupUUID).First(&group).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get group by uuid: %w", err)
	}
	return &group, nil
}

func (r *GroupRepository) GetMember(groupUUID, userUUID string) (*model.GroupMember, error) {
	groupUUID = strings.TrimSpace(groupUUID)
	userUUID = strings.TrimSpace(userUUID)
	if groupUUID == "" || userUUID == "" {
		return nil, nil
	}
	var member model.GroupMember
	if err := r.database().Where("group_uuid = ? AND user_uuid = ?", groupUUID, userUUID).First(&member).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get group member: %w", err)
	}
	return &member, nil
}

func (r *GroupRepository) ListMembers(groupUUID string) ([]*model.GroupMember, error) {
	groupUUID = strings.TrimSpace(groupUUID)
	if groupUUID == "" {
		return []*model.GroupMember{}, nil
	}
	var members []*model.GroupMember
	if err := r.database().Where("group_uuid = ?", groupUUID).Order("role ASC, joined_at ASC").Find(&members).Error; err != nil {
		return nil, fmt.Errorf("list group members: %w", err)
	}
	return members, nil
}

func (r *GroupRepository) AddMembers(groupUUID string, members []*model.GroupMember) error {
	valid := uniqueGroupMembers(members)
	if len(valid) == 0 {
		return nil
	}
	return r.database().Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&valid)
		if result.Error != nil {
			return fmt.Errorf("add group members: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return nil
		}
		if err := tx.Model(&model.Group{}).
			Where("uuid = ?", groupUUID).
			UpdateColumn("member_count", gorm.Expr("member_count + ?", result.RowsAffected)).Error; err != nil {
			return fmt.Errorf("increase group member count: %w", err)
		}
		return nil
	})
}

func (r *GroupRepository) Update(group *model.Group) error {
	if err := r.database().Save(group).Error; err != nil {
		return fmt.Errorf("update group: %w", err)
	}
	return nil
}

func (r *GroupRepository) RemoveMembers(groupUUID string, memberUUIDs []string) error {
	if len(memberUUIDs) == 0 {
		return nil
	}
	return r.database().Transaction(func(tx *gorm.DB) error {
		result := tx.Where("group_uuid = ? AND user_uuid IN ?", groupUUID, memberUUIDs).Delete(&model.GroupMember{})
		if result.Error != nil {
			return fmt.Errorf("remove group members: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return nil
		}
		if err := tx.Model(&model.Group{}).
			Where("uuid = ?", groupUUID).
			UpdateColumn("member_count", gorm.Expr("member_count - ?", result.RowsAffected)).Error; err != nil {
			return fmt.Errorf("decrease group member count by batch: %w", err)
		}
		return nil
	})
}

func (r *GroupRepository) RemoveMember(groupUUID, userUUID string) error {
	return r.database().Transaction(func(tx *gorm.DB) error {
		result := tx.Where("group_uuid = ? AND user_uuid = ?", groupUUID, userUUID).Delete(&model.GroupMember{})
		if result.Error != nil {
			return fmt.Errorf("remove group member: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return nil
		}
		if err := tx.Model(&model.Group{}).
			Where("uuid = ?", groupUUID).
			UpdateColumn("member_count", gorm.Expr("member_count - 1")).Error; err != nil {
			return fmt.Errorf("decrease group member count: %w", err)
		}
		return nil
	})
}

func nonNilGroupMembers(members []*model.GroupMember) []*model.GroupMember {
	valid := make([]*model.GroupMember, 0, len(members))
	for _, member := range members {
		if member != nil {
			valid = append(valid, member)
		}
	}
	return valid
}

func uniqueGroupMembers(members []*model.GroupMember) []*model.GroupMember {
	seen := make(map[string]struct{}, len(members))
	unique := make([]*model.GroupMember, 0, len(members))
	for _, member := range members {
		if member == nil {
			continue
		}
		key := member.GroupUUID + "\x00" + member.UserUUID
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, member)
	}
	return unique
}

func (r *GroupRepository) database() *gorm.DB {
	if r != nil && r.db != nil {
		return r.db
	}
	return store.DB
}
