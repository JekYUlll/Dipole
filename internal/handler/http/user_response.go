package http

import (
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/service"
)

type publicUserResponse struct {
	UUID     string `json:"uuid"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Status   int8   `json:"status"`
}

type privateUserResponse struct {
	ID        uint      `json:"id"`
	UUID      string    `json:"uuid"`
	Nickname  string    `json:"nickname"`
	Telephone string    `json:"telephone"`
	Email     string    `json:"email"`
	Avatar    string    `json:"avatar"`
	IsAdmin   bool      `json:"is_admin"`
	Status    int8      `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type authResponse struct {
	Token string               `json:"token"`
	User  *privateUserResponse `json:"user"`
}

func presentUserForViewer(viewer *model.User, target *model.User) any {
	if canViewPrivateUser(viewer, target) {
		return toPrivateUserResponse(target)
	}

	return toPublicUserResponse(target)
}

func presentUsersForViewer(viewer *model.User, users []*model.User) any {
	if viewer != nil && viewer.IsAdmin {
		response := make([]*privateUserResponse, 0, len(users))
		for _, user := range users {
			response = append(response, toPrivateUserResponse(user))
		}
		return response
	}

	response := make([]*publicUserResponse, 0, len(users))
	for _, user := range users {
		response = append(response, toPublicUserResponse(user))
	}
	return response
}

func canViewPrivateUser(viewer *model.User, target *model.User) bool {
	if viewer == nil || target == nil {
		return false
	}

	return viewer.IsAdmin || viewer.UUID == target.UUID
}

func toPublicUserResponse(user *model.User) *publicUserResponse {
	if user == nil {
		return nil
	}

	return &publicUserResponse{
		UUID:     user.UUID,
		Nickname: user.Nickname,
		Avatar:   user.Avatar,
		Status:   user.Status,
	}
}

func toPrivateUserResponse(user *model.User) *privateUserResponse {
	if user == nil {
		return nil
	}

	return &privateUserResponse{
		ID:        user.ID,
		UUID:      user.UUID,
		Nickname:  user.Nickname,
		Telephone: user.Telephone,
		Email:     user.Email,
		Avatar:    user.Avatar,
		IsAdmin:   user.IsAdmin,
		Status:    user.Status,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

func newAuthResponse(result *service.AuthResult) *authResponse {
	if result == nil {
		return nil
	}

	return &authResponse{
		Token: result.Token,
		User:  toPrivateUserResponse(result.User),
	}
}
