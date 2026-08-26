package mapper

import (
	"database/sql"
	"time"

	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	"github.com/JekYUlll/Dipole/internal/model"
)

func GroupCreateParams(group *model.Group) generated.CreateGroupParams {
	now := time.Now().UTC()
	createdAt := group.CreatedAt
	if createdAt.IsZero() {
		createdAt = now
	}
	updatedAt := group.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}
	return generated.CreateGroupParams{
		Uuid:           group.UUID,
		Name:           group.Name,
		Notice:         group.Notice,
		Avatar:         group.Avatar,
		AvatarFileUuid: groupString(group.AvatarFileUUID),
		OwnerUuid:      group.OwnerUUID,
		MemberCount:    int64(group.MemberCount),
		Status:         group.Status,
		CreatedAt:      groupTime(createdAt),
		UpdatedAt:      groupTime(updatedAt),
	}
}

func GroupUpdateParams(group *model.Group) generated.UpdateGroupParams {
	return generated.UpdateGroupParams{
		Name:           group.Name,
		Notice:         group.Notice,
		Avatar:         group.Avatar,
		AvatarFileUuid: groupString(group.AvatarFileUUID),
		OwnerUuid:      group.OwnerUUID,
		MemberCount:    int64(group.MemberCount),
		Status:         group.Status,
		Uuid:           group.UUID,
	}
}

func GroupMemberCreateParams(member *model.GroupMember) generated.CreateGroupMemberParams {
	joinedAt, createdAt, updatedAt := groupMemberTimes(member)
	return generated.CreateGroupMemberParams{
		GroupUuid: member.GroupUUID,
		UserUuid:  member.UserUUID,
		Role:      member.Role,
		JoinedAt:  joinedAt,
		CreatedAt: groupTime(createdAt),
		UpdatedAt: groupTime(updatedAt),
	}
}

func GroupMemberAddParams(member *model.GroupMember) generated.AddGroupMemberParams {
	joinedAt, createdAt, updatedAt := groupMemberTimes(member)
	return generated.AddGroupMemberParams{
		GroupUuid: member.GroupUUID,
		UserUuid:  member.UserUUID,
		Role:      member.Role,
		JoinedAt:  joinedAt,
		CreatedAt: groupTime(createdAt),
		UpdatedAt: groupTime(updatedAt),
	}
}

func groupMemberTimes(member *model.GroupMember) (time.Time, time.Time, time.Time) {
	now := time.Now().UTC()
	joinedAt := member.JoinedAt
	if joinedAt.IsZero() {
		joinedAt = now
	}
	createdAt := member.CreatedAt
	if createdAt.IsZero() {
		createdAt = now
	}
	updatedAt := member.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}
	return joinedAt, createdAt, updatedAt
}

func Group(row generated.Group) *model.Group {
	return &model.Group{
		ID:             uint(row.ID),
		UUID:           row.Uuid,
		Name:           row.Name,
		Notice:         row.Notice,
		Avatar:         row.Avatar,
		AvatarFileUUID: row.AvatarFileUuid.String,
		OwnerUUID:      row.OwnerUuid,
		MemberCount:    int(row.MemberCount),
		Status:         row.Status,
		CreatedAt:      row.CreatedAt.Time,
		UpdatedAt:      row.UpdatedAt.Time,
	}
}

func GroupMember(row generated.GroupMember) *model.GroupMember {
	return &model.GroupMember{
		ID:        uint(row.ID),
		GroupUUID: row.GroupUuid,
		UserUUID:  row.UserUuid,
		Role:      row.Role,
		JoinedAt:  row.JoinedAt,
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}
}

func GroupMembers(rows []generated.GroupMember) []*model.GroupMember {
	members := make([]*model.GroupMember, 0, len(rows))
	for _, row := range rows {
		members = append(members, GroupMember(row))
	}
	return members
}

func groupString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: true}
}

func groupTime(value time.Time) sql.NullTime {
	return sql.NullTime{Time: value, Valid: true}
}
