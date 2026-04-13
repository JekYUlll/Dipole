package httpdto

import (
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/service"
)

type RegisterRequest struct {
	Nickname  string `json:"nickname" binding:"required,min=2,max=20"`
	Telephone string `json:"telephone" binding:"required,len=11"`
	Password  string `json:"password" binding:"required,min=6,max=32"`
	Email     string `json:"email" binding:"omitempty,email,max=64"`
}

func (r RegisterRequest) ToInput() service.RegisterInput {
	return service.RegisterInput{
		Nickname:  r.Nickname,
		Telephone: r.Telephone,
		Password:  r.Password,
		Email:     r.Email,
	}
}

type LoginRequest struct {
	Telephone string `json:"telephone" binding:"required,len=11"`
	Password  string `json:"password" binding:"required,min=6,max=32"`
}

func (r LoginRequest) ToInput() service.LoginInput {
	return service.LoginInput{
		Telephone: r.Telephone,
		Password:  r.Password,
	}
}

type AuthResponse struct {
	Token string               `json:"token"`
	User  *PrivateUserResponse `json:"user"`
}

func NewAuthResponse(result *service.AuthResult) *AuthResponse {
	if result == nil {
		return nil
	}

	return &AuthResponse{
		Token: result.Token,
		User:  ToPrivateUserResponse(result.User),
	}
}

type PublicUserResponse struct {
	UUID     string `json:"uuid"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	UserType int8   `json:"user_type"`
	Status   int8   `json:"status"`
}

type PrivateUserResponse struct {
	ID        uint      `json:"id"`
	UUID      string    `json:"uuid"`
	Nickname  string    `json:"nickname"`
	Telephone string    `json:"telephone"`
	Email     string    `json:"email"`
	Avatar    string    `json:"avatar"`
	IsAdmin   bool      `json:"is_admin"`
	UserType  int8      `json:"user_type"`
	Status    int8      `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func PresentUserForViewer(viewer *model.User, target *model.User) any {
	if CanViewPrivateUser(viewer, target) {
		return ToPrivateUserResponse(target)
	}

	return ToPublicUserResponse(target)
}

func PresentUsersForViewer(viewer *model.User, users []*model.User) any {
	if viewer != nil && viewer.IsAdmin {
		response := make([]*PrivateUserResponse, 0, len(users))
		for _, user := range users {
			response = append(response, ToPrivateUserResponse(user))
		}
		return response
	}

	response := make([]*PublicUserResponse, 0, len(users))
	for _, user := range users {
		response = append(response, ToPublicUserResponse(user))
	}
	return response
}

func CanViewPrivateUser(viewer *model.User, target *model.User) bool {
	if viewer == nil || target == nil {
		return false
	}

	return viewer.IsAdmin || viewer.UUID == target.UUID
}

func ToPublicUserResponse(user *model.User) *PublicUserResponse {
	if user == nil {
		return nil
	}

	return &PublicUserResponse{
		UUID:     user.UUID,
		Nickname: user.Nickname,
		Avatar:   user.Avatar,
		UserType: user.UserType,
		Status:   user.Status,
	}
}

func ToPrivateUserResponse(user *model.User) *PrivateUserResponse {
	if user == nil {
		return nil
	}

	return &PrivateUserResponse{
		ID:        user.ID,
		UUID:      user.UUID,
		Nickname:  user.Nickname,
		Telephone: user.Telephone,
		Email:     user.Email,
		Avatar:    user.Avatar,
		IsAdmin:   user.IsAdmin,
		UserType:  user.UserType,
		Status:    user.Status,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}
