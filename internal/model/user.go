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
	ID             uint      `json:"id"`
	UUID           string    `json:"uuid"`
	Nickname       string    `json:"nickname"`
	Telephone      string    `json:"telephone"`
	Email          string    `json:"email"`
	Avatar         string    `json:"avatar"`
	AvatarFileUUID string    `json:"avatar_file_uuid,omitempty"`
	Signature      string    `json:"signature"`
	PasswordHash   string    `json:"-"`
	IsAdmin        bool      `json:"is_admin"`
	UserType       int8      `json:"user_type"`
	Status         int8      `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (u *User) IsAssistant() bool {
	return u != nil && u.UserType == UserTypeAssistant
}
