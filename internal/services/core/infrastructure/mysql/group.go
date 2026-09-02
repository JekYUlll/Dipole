package coremysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/mapper"
	"github.com/JekYUlll/Dipole/internal/model"
)

var _ application.GroupStore = (*GroupRepository)(nil)

type GroupRepository struct {
	store transactionStore
}

func NewGroupRepository(store transactionStore) (*GroupRepository, error) {
	if store == nil {
		return nil, errors.New("group transaction store is required")
	}
	return &GroupRepository{store: store}, nil
}

func (r *GroupRepository) Create(group *model.Group, members []*model.GroupMember) error {
	if group == nil {
		return errors.New("create group with sqlc: group is required")
	}
	ctx := context.Background()
	if err := r.store.WithinTx(ctx, nil, func(queries *generated.Queries) error {
		if _, err := queries.CreateGroup(ctx, mapper.GroupCreateParams(group)); err != nil {
			return fmt.Errorf("create group with sqlc: %w", err)
		}
		for _, member := range members {
			if member == nil {
				continue
			}
			if _, err := queries.CreateGroupMember(ctx, mapper.GroupMemberCreateParams(member)); err != nil {
				return fmt.Errorf("create group member with sqlc: %w", err)
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return r.reload(group)
}

func (r *GroupRepository) GetByUUID(groupUUID string) (*model.Group, error) {
	groupUUID = strings.TrimSpace(groupUUID)
	if groupUUID == "" {
		return nil, nil
	}
	row, err := r.store.Queries().GetGroupByUUID(context.Background(), groupUUID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get group by UUID with sqlc: %w", err)
	}
	return mapper.Group(row), nil
}

func (r *GroupRepository) GetMember(groupUUID, userUUID string) (*model.GroupMember, error) {
	groupUUID = strings.TrimSpace(groupUUID)
	userUUID = strings.TrimSpace(userUUID)
	if groupUUID == "" || userUUID == "" {
		return nil, nil
	}
	row, err := r.store.Queries().GetGroupMember(context.Background(), generated.GetGroupMemberParams{
		GroupUuid: groupUUID,
		UserUuid:  userUUID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get group member with sqlc: %w", err)
	}
	return mapper.GroupMember(row), nil
}

func (r *GroupRepository) ListMembers(groupUUID string) ([]*model.GroupMember, error) {
	groupUUID = strings.TrimSpace(groupUUID)
	if groupUUID == "" {
		return []*model.GroupMember{}, nil
	}
	rows, err := r.store.Queries().ListGroupMembers(context.Background(), groupUUID)
	if err != nil {
		return nil, fmt.Errorf("list group members with sqlc: %w", err)
	}
	return mapper.GroupMembers(rows), nil
}

func (r *GroupRepository) AddMembers(groupUUID string, members []*model.GroupMember) error {
	valid := validSQLCGroupMembers(members)
	if len(valid) == 0 {
		return nil
	}
	ctx := context.Background()
	return r.store.WithinTx(ctx, nil, func(queries *generated.Queries) error {
		var inserted int64
		for _, member := range valid {
			result, err := queries.AddGroupMember(ctx, mapper.GroupMemberAddParams(member))
			if err != nil {
				return fmt.Errorf("add group member with sqlc: %w", err)
			}
			rows, err := result.RowsAffected()
			if err != nil {
				return fmt.Errorf("read inserted group member count: %w", err)
			}
			inserted += rows
		}
		if inserted == 0 {
			return nil
		}
		_, err := queries.AdjustGroupMemberCount(ctx, generated.AdjustGroupMemberCountParams{
			MemberCount: inserted,
			Uuid:        groupUUID,
		})
		if err != nil {
			return fmt.Errorf("increase group member count with sqlc: %w", err)
		}
		return nil
	})
}

func (r *GroupRepository) Update(group *model.Group) error {
	if group == nil {
		return errors.New("update group with sqlc: group is required")
	}
	if _, err := r.store.Queries().UpdateGroup(context.Background(), mapper.GroupUpdateParams(group)); err != nil {
		return fmt.Errorf("update group with sqlc: %w", err)
	}
	return r.reload(group)
}

func (r *GroupRepository) RemoveMembers(groupUUID string, memberUUIDs []string) error {
	if len(memberUUIDs) == 0 {
		return nil
	}
	ctx := context.Background()
	return r.store.WithinTx(ctx, nil, func(queries *generated.Queries) error {
		result, err := queries.DeleteGroupMembers(ctx, generated.DeleteGroupMembersParams{
			GroupUuid: groupUUID,
			UserUuids: memberUUIDs,
		})
		if err != nil {
			return fmt.Errorf("remove group members with sqlc: %w", err)
		}
		removed, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("read removed group member count: %w", err)
		}
		return adjustRemovedGroupMembers(ctx, queries, groupUUID, removed)
	})
}

func (r *GroupRepository) RemoveMember(groupUUID, userUUID string) error {
	ctx := context.Background()
	return r.store.WithinTx(ctx, nil, func(queries *generated.Queries) error {
		result, err := queries.DeleteGroupMember(ctx, generated.DeleteGroupMemberParams{
			GroupUuid: groupUUID,
			UserUuid:  userUUID,
		})
		if err != nil {
			return fmt.Errorf("remove group member with sqlc: %w", err)
		}
		removed, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("read removed group member count: %w", err)
		}
		return adjustRemovedGroupMembers(ctx, queries, groupUUID, removed)
	})
}

func (r *GroupRepository) reload(group *model.Group) error {
	stored, err := r.GetByUUID(group.UUID)
	if err != nil {
		return err
	}
	if stored == nil {
		return errors.New("persisted group was not found")
	}
	*group = *stored
	return nil
}

func adjustRemovedGroupMembers(ctx context.Context, queries *generated.Queries, groupUUID string, removed int64) error {
	if removed == 0 {
		return nil
	}
	_, err := queries.AdjustGroupMemberCount(ctx, generated.AdjustGroupMemberCountParams{
		MemberCount: -removed,
		Uuid:        groupUUID,
	})
	if err != nil {
		return fmt.Errorf("decrease group member count with sqlc: %w", err)
	}
	return nil
}

func validSQLCGroupMembers(members []*model.GroupMember) []*model.GroupMember {
	valid := make([]*model.GroupMember, 0, len(members))
	for _, member := range members {
		if member != nil {
			valid = append(valid, member)
		}
	}
	return valid
}
