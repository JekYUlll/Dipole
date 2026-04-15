package ws

import (
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/service"
)

// SessionUser carries only the user identity needed during a websocket session.
type SessionUser struct {
	UUID           string
	IsAdmin        bool
	TokenID        string
	TokenExpiresAt time.Time
}

func newSessionUser(user *model.User, session *service.TokenSession) *SessionUser {
	if user == nil {
		return nil
	}

	sessionUser := &SessionUser{
		UUID:    user.UUID,
		IsAdmin: user.IsAdmin,
	}
	if session != nil {
		sessionUser.TokenID = session.TokenID
		sessionUser.TokenExpiresAt = session.ExpiresAt
	}

	return sessionUser
}
