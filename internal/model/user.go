package model

import "time"

const (
	DefaultAvatarURL = "https://cube.elemecdn.com/0/88/03b0d39583f48206768a7534e55bcpng.png"

	UserStatusNormal int8 = iota
	UserStatusDisabled
)

const (
	UserTypeNormal int8 = iota
	UserTypeAssistant
)

type User struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	UUID           string    `gorm:"size:24;uniqueIndex;not null" json:"uuid"`
	Nickname       string    `gorm:"size:20;not null" json:"nickname"`
	Telephone      string    `gorm:"size:11;uniqueIndex;not null" json:"telephone"`
	Email          string    `gorm:"size:64" json:"email"`
	Avatar         string    `gorm:"size:255;not null;default:''" json:"avatar"`
	AvatarFileUUID string    `gorm:"column:avatar_file_uuid;size:24;index" json:"avatar_file_uuid,omitempty"`
	PasswordHash   string    `gorm:"column:password_hash;size:255;not null" json:"-"`
	IsAdmin        bool      `gorm:"not null;default:false" json:"is_admin"`
	UserType       int8      `gorm:"column:user_type;not null;default:0;index" json:"user_type"`
	Status         int8      `gorm:"not null;default:0" json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (User) TableName() string {
	return "users"
}

func (u *User) IsAssistant() bool {
	return u != nil && u.UserType == UserTypeAssistant
}
