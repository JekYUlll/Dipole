package model

import "time"

const (
	ContactApplicationPending int8 = iota
	ContactApplicationAccepted
	ContactApplicationRejected
	ContactApplicationExpired
)

const (
	ContactStatusNormal int8 = iota
	ContactStatusBlocked
)

type Contact struct {
	ID         uint      `json:"id"`
	UserUUID   string    `json:"user_uuid"`
	FriendUUID string    `json:"friend_uuid"`
	Remark     string    `json:"remark"`
	Status     int8      `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type ContactApplication struct {
	ID            uint       `json:"id"`
	ApplicantUUID string     `json:"applicant_uuid"`
	TargetUUID    string     `json:"target_uuid"`
	Message       string     `json:"message"`
	Status        int8       `json:"status"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	HandledAt     *time.Time `json:"handled_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
