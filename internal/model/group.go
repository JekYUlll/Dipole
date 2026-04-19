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
	ID          uint      `gorm:"primaryKey" json:"id"`
	UUID        string    `gorm:"size:24;uniqueIndex;not null" json:"uuid"`
	Name        string    `gorm:"size:50;not null" json:"name"`
	Notice      string    `gorm:"size:500;not null;default:''" json:"notice"`
	Avatar      string    `gorm:"size:255;not null;default:''" json:"avatar"`
	OwnerUUID   string    `gorm:"column:owner_uuid;size:24;index;not null" json:"owner_uuid"`
	MemberCount int       `gorm:"column:member_count;not null;default:1" json:"member_count"`
	Status      int8      `gorm:"not null;default:0;index" json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (Group) TableName() string {
	return "groups"
}

type GroupMember struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	GroupUUID  string    `gorm:"column:group_uuid;size:24;not null;uniqueIndex:idx_group_user,priority:1;index" json:"group_uuid"`
	UserUUID   string    `gorm:"column:user_uuid;size:24;not null;uniqueIndex:idx_group_user,priority:2;index" json:"user_uuid"`
	Role       int8      `gorm:"not null" json:"role"`
	JoinedAt   time.Time `gorm:"column:joined_at;not null" json:"joined_at"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (GroupMember) TableName() string {
	return "group_members"
}
