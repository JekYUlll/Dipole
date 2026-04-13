package ws

import "github.com/JekYUlll/Dipole/internal/model"

// SessionUser carries only the user identity needed during a websocket session.
type SessionUser struct {
	UUID    string
	IsAdmin bool
}

func newSessionUser(user *model.User) *SessionUser {
	if user == nil {
		return nil
	}

	return &SessionUser{
		UUID:    user.UUID,
		IsAdmin: user.IsAdmin,
	}
}
