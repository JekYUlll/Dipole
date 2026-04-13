package repository

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/store"
)

type GroupRepository struct{}

func NewGroupRepository() *GroupRepository {
	return &GroupRepository{}
}

func (r *GroupRepository) Create(group *model.Group, members []*model.GroupMember) error {
	return store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(group).Error; err != nil {
			return fmt.Errorf("create group: %w", err)
		}
		if len(members) == 0 {
			return nil
		}
		if err := tx.Create(&members).Error; err != nil {
			return fmt.Errorf("create group members: %w", err)
		}

		return nil
	})
}

func (r *GroupRepository) GetByUUID(groupUUID string) (*model.Group, error) {
	var group model.Group
	if err := store.DB.Where("uuid = ?", groupUUID).First(&group).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, fmt.Errorf("get group by uuid: %w", err)
	}

	return &group, nil
}

func (r *GroupRepository) GetMember(groupUUID, userUUID string) (*model.GroupMember, error) {
	var member model.GroupMember
	if err := store.DB.Where("group_uuid = ? AND user_uuid = ?", groupUUID, userUUID).First(&member).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, fmt.Errorf("get group member: %w", err)
	}

	return &member, nil
}

func (r *GroupRepository) ListMembers(groupUUID string) ([]*model.GroupMember, error) {
	var members []*model.GroupMember
	if err := store.DB.Where("group_uuid = ?", groupUUID).Order("role ASC, joined_at ASC").Find(&members).Error; err != nil {
		return nil, fmt.Errorf("list group members: %w", err)
	}

	return members, nil
}

func (r *GroupRepository) AddMembers(groupUUID string, members []*model.GroupMember) error {
	if len(members) == 0 {
		return nil
	}

	return store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&members).Error; err != nil {
			return fmt.Errorf("add group members: %w", err)
		}
		if err := tx.Model(&model.Group{}).
			Where("uuid = ?", groupUUID).
			UpdateColumn("member_count", gorm.Expr("member_count + ?", len(members))).Error; err != nil {
			return fmt.Errorf("increase group member count: %w", err)
		}

		return nil
	})
}

func (r *GroupRepository) Update(group *model.Group) error {
	if err := store.DB.Save(group).Error; err != nil {
		return fmt.Errorf("update group: %w", err)
	}

	return nil
}

func (r *GroupRepository) RemoveMembers(groupUUID string, memberUUIDs []string) error {
	if len(memberUUIDs) == 0 {
		return nil
	}

	return store.DB.Transaction(func(tx *gorm.DB) error {
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
	return store.DB.Transaction(func(tx *gorm.DB) error {
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
