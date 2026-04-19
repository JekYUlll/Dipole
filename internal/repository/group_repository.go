package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/JekYUlll/Dipole/internal/model"
	platformBloom "github.com/JekYUlll/Dipole/internal/platform/bloom"
	platformCache "github.com/JekYUlll/Dipole/internal/platform/cache"
	"github.com/JekYUlll/Dipole/internal/store"
)

type GroupRepository struct{}

func NewGroupRepository() *GroupRepository {
	return &GroupRepository{}
}

func (r *GroupRepository) Create(group *model.Group, members []*model.GroupMember) error {
	if err := store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(group).Error; err != nil {
			return fmt.Errorf("create group: %w", err)
		}
		if len(members) == 0 {
			return nil
		}

		rows := make([]map[string]any, 0, len(members))
		for _, member := range members {
			if member == nil {
				continue
			}
			rows = append(rows, map[string]any{
				"group_uuid": member.GroupUUID,
				"user_uuid":  member.UserUUID,
				"role":       member.Role,
				"joined_at":  member.JoinedAt,
				"created_at": member.CreatedAt,
				"updated_at": member.UpdatedAt,
			})
		}
		if len(rows) == 0 {
			return nil
		}

		if err := tx.Table(model.GroupMember{}.TableName()).Create(rows).Error; err != nil {
			return fmt.Errorf("create group members: %w", err)
		}

		return nil
	}); err != nil {
		return err
	}
	if group != nil {
		platformBloom.AddGroup(group.UUID)
	}

	ctx, cancel := platformCache.NewContext()
	defer cancel()

	if group != nil {
		_ = platformCache.SetJSON(ctx, platformCache.GroupMetaKey(group.UUID), group, platformCache.GroupMetaTTL)
	}
	if len(members) > 0 && group != nil {
		for _, member := range members {
			if member == nil {
				continue
			}
			_ = platformCache.HashSetJSON(ctx, platformCache.GroupMembersKey(group.UUID), member.UserUUID, member)
		}
		_ = platformCache.Expire(ctx, platformCache.GroupMembersKey(group.UUID), platformCache.GroupMembersTTL)
	}

	return nil
}

func (r *GroupRepository) GetByUUID(groupUUID string) (*model.Group, error) {
	groupUUID = strings.TrimSpace(groupUUID)
	if groupUUID == "" {
		return nil, nil
	}
	if !platformBloom.GroupMayExist(groupUUID) {
		return nil, nil
	}

	ctx, cancel := platformCache.NewContext()
	defer cancel()

	var cached model.Group
	if hit, err := platformCache.GetJSON(ctx, platformCache.GroupMetaKey(groupUUID), &cached); err == nil && hit {
		return &cached, nil
	}

	var group model.Group
	if err := store.DB.Where("uuid = ?", groupUUID).First(&group).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, fmt.Errorf("get group by uuid: %w", err)
	}

	_ = platformCache.SetJSON(ctx, platformCache.GroupMetaKey(groupUUID), &group, platformCache.GroupMetaTTL)
	return &group, nil
}

func (r *GroupRepository) GetMember(groupUUID, userUUID string) (*model.GroupMember, error) {
	groupUUID = strings.TrimSpace(groupUUID)
	userUUID = strings.TrimSpace(userUUID)
	if groupUUID == "" || userUUID == "" {
		return nil, nil
	}
	if !platformBloom.GroupMayExist(groupUUID) {
		return nil, nil
	}

	ctx, cancel := platformCache.NewContext()
	defer cancel()

	var cached model.GroupMember
	if hit, err := platformCache.HashGetJSON(ctx, platformCache.GroupMembersKey(groupUUID), userUUID, &cached); err == nil && hit {
		return &cached, nil
	}

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
	groupUUID = strings.TrimSpace(groupUUID)
	if groupUUID == "" {
		return []*model.GroupMember{}, nil
	}
	if !platformBloom.GroupMayExist(groupUUID) {
		return []*model.GroupMember{}, nil
	}

	ctx, cancel := platformCache.NewContext()
	defer cancel()

	if cachedMembers, err := platformCache.HashGetAll(ctx, platformCache.GroupMembersKey(groupUUID)); err == nil && len(cachedMembers) > 0 {
		members := make([]*model.GroupMember, 0, len(cachedMembers))
		for _, raw := range cachedMembers {
			var member model.GroupMember
			if err := json.Unmarshal([]byte(raw), &member); err != nil {
				members = nil
				break
			}
			members = append(members, &member)
		}
		if len(members) > 0 {
			sortGroupMembers(members)
			return members, nil
		}
	}

	var members []*model.GroupMember
	if err := store.DB.Where("group_uuid = ?", groupUUID).Order("role ASC, joined_at ASC").Find(&members).Error; err != nil {
		return nil, fmt.Errorf("list group members: %w", err)
	}

	for _, member := range members {
		if member == nil {
			continue
		}
		_ = platformCache.HashSetJSON(ctx, platformCache.GroupMembersKey(groupUUID), member.UserUUID, member)
	}
	_ = platformCache.Expire(ctx, platformCache.GroupMembersKey(groupUUID), platformCache.GroupMembersTTL)

	return members, nil
}

func (r *GroupRepository) AddMembers(groupUUID string, members []*model.GroupMember) error {
	if len(members) == 0 {
		return nil
	}

	if err := store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&members).Error; err != nil {
			return fmt.Errorf("add group members: %w", err)
		}
		if err := tx.Model(&model.Group{}).
			Where("uuid = ?", groupUUID).
			UpdateColumn("member_count", gorm.Expr("member_count + ?", len(members))).Error; err != nil {
			return fmt.Errorf("increase group member count: %w", err)
		}

		return nil
	}); err != nil {
		return err
	}

	r.invalidateGroupCache(groupUUID)
	return nil
}

func (r *GroupRepository) Update(group *model.Group) error {
	if err := store.DB.Save(group).Error; err != nil {
		return fmt.Errorf("update group: %w", err)
	}

	ctx, cancel := platformCache.NewContext()
	defer cancel()

	if group != nil {
		_ = platformCache.SetJSON(ctx, platformCache.GroupMetaKey(group.UUID), group, platformCache.GroupMetaTTL)
		if group.Status == model.GroupStatusDismissed {
			_ = platformCache.Delete(ctx, platformCache.GroupMembersKey(group.UUID))
		}
	}

	return nil
}

func (r *GroupRepository) RemoveMembers(groupUUID string, memberUUIDs []string) error {
	if len(memberUUIDs) == 0 {
		return nil
	}

	if err := store.DB.Transaction(func(tx *gorm.DB) error {
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
	}); err != nil {
		return err
	}

	r.invalidateGroupCache(groupUUID)
	return nil
}

func (r *GroupRepository) RemoveMember(groupUUID, userUUID string) error {
	if err := store.DB.Transaction(func(tx *gorm.DB) error {
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
	}); err != nil {
		return err
	}

	r.invalidateGroupCache(groupUUID)
	return nil
}

func (r *GroupRepository) invalidateGroupCache(groupUUID string) {
	ctx, cancel := platformCache.NewContext()
	defer cancel()

	_ = platformCache.Delete(
		ctx,
		platformCache.GroupMetaKey(groupUUID),
		platformCache.GroupMembersKey(groupUUID),
	)
}

func sortGroupMembers(members []*model.GroupMember) {
	sort.Slice(members, func(i, j int) bool {
		if members[i].Role != members[j].Role {
			return members[i].Role < members[j].Role
		}
		if !members[i].JoinedAt.Equal(members[j].JoinedAt) {
			return members[i].JoinedAt.Before(members[j].JoinedAt)
		}
		return members[i].UserUUID < members[j].UserUUID
	})
}
