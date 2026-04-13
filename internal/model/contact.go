package model

import "time"

const (
	ContactApplicationPending int8 = iota
	ContactApplicationAccepted
	ContactApplicationRejected
)

const (
	ContactStatusNormal int8 = iota
	ContactStatusBlocked
)

type Contact struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserUUID   string    `gorm:"column:user_uuid;size:24;not null;uniqueIndex:idx_user_friend,priority:1" json:"user_uuid"`
	FriendUUID string    `gorm:"column:friend_uuid;size:24;not null;uniqueIndex:idx_user_friend,priority:2;index" json:"friend_uuid"`
	Remark     string    `gorm:"size:50;not null;default:''" json:"remark"`
	Status     int8      `gorm:"not null;default:0;index" json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (Contact) TableName() string {
	return "contacts"
}

type ContactApplication struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	ApplicantUUID string     `gorm:"column:applicant_uuid;size:24;not null;uniqueIndex:idx_applicant_target,priority:1" json:"applicant_uuid"`
	TargetUUID    string     `gorm:"column:target_uuid;size:24;not null;uniqueIndex:idx_applicant_target,priority:2;index" json:"target_uuid"`
	Message       string     `gorm:"size:255;not null;default:''" json:"message"`
	Status        int8       `gorm:"not null;default:0;index" json:"status"`
	HandledAt     *time.Time `gorm:"column:handled_at" json:"handled_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (ContactApplication) TableName() string {
	return "contact_applications"
}
