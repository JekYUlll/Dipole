package model

import "time"

const (
	GroupStatusNormal int8 = iota
	GroupStatusDismissed
)

const (
	GroupMemberRoleOwner int8 = iota
	GroupMemberRoleMember
)

type Group struct {
	ID             uint      `json:"id"`
	UUID           string    `json:"uuid"`
	Name           string    `json:"name"`
	Notice         string    `json:"notice"`
	Avatar         string    `json:"avatar"`
	AvatarFileUUID string    `json:"avatar_file_uuid,omitempty"`
	OwnerUUID      string    `json:"owner_uuid"`
	MemberCount    int       `json:"member_count"`
	Status         int8      `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type GroupMember struct {
	ID        uint      `json:"id"`
	GroupUUID string    `json:"group_uuid"`
	UserUUID  string    `json:"user_uuid"`
	Role      int8      `json:"role"`
	JoinedAt  time.Time `json:"joined_at"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
