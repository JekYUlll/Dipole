package ws

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/JekYUlll/Dipole/internal/model"
)

var (
	ErrTokenRequired      = errors.New("authorization token is required")
	ErrTokenInvalid       = errors.New("authorization token is invalid")
	ErrUserSessionInvalid = errors.New("user session is invalid")
)

type tokenResolver interface {
	Resolve(token string) (string, error)
}

type userFinder interface {
	GetByUUID(uuid string) (*model.User, error)
}

type Authenticator struct {
	tokenResolver tokenResolver
	userFinder    userFinder
}

func NewAuthenticator(tokenResolver tokenResolver, userFinder userFinder) *Authenticator {
	return &Authenticator{
		tokenResolver: tokenResolver,
		userFinder:    userFinder,
	}
}

func (a *Authenticator) Authenticate(r *http.Request) (*SessionUser, string, error) {
	token, ok := extractAccessToken(r)
	if !ok {
		return nil, "", ErrTokenRequired
	}

	userUUID, err := a.tokenResolver.Resolve(token)
	if err != nil {
		return nil, "", ErrTokenInvalid
	}

	user, err := a.userFinder.GetByUUID(userUUID)
	if err != nil {
		return nil, "", fmt.Errorf("get user by uuid: %w", err)
	}
	if user == nil || user.Status == model.UserStatusDisabled {
		return nil, "", ErrUserSessionInvalid
	}

	return newSessionUser(user), token, nil
}

func extractAccessToken(r *http.Request) (string, bool) {
	for _, key := range []string{"token", "access_token"} {
		token := strings.TrimSpace(r.URL.Query().Get(key))
		if token != "" {
			return token, true
		}
	}

	return parseBearerToken(r.Header.Get("Authorization"))
}

func parseBearerToken(header string) (string, bool) {
	header = strings.TrimSpace(header)
	if header == "" {
		return "", false
	}

	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}

	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", false
	}

	return token, true
}
