package httpdto

import "github.com/JekYUlll/Dipole/internal/service"

type UpdateProfileRequest struct {
	Nickname *string `json:"nickname"`
	Email    *string `json:"email"`
	Avatar   *string `json:"avatar"`
}

func (r UpdateProfileRequest) ToInput() service.UpdateProfileInput {
	return service.UpdateProfileInput{
		Nickname: r.Nickname,
		Email:    r.Email,
		Avatar:   r.Avatar,
	}
}

type UpdateStatusRequest struct {
	Status int8 `json:"status"`
}
