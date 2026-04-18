package httpdto

import "github.com/JekYUlll/Dipole/internal/service"

type UpdateProfileRequest struct {
	Nickname  *string `json:"nickname"`
	Email     *string `json:"email"`
	Avatar    *string `json:"avatar"`
	Signature *string `json:"signature"`
}

func (r UpdateProfileRequest) ToInput() service.UpdateProfileInput {
	return service.UpdateProfileInput{
		Nickname:  r.Nickname,
		Email:     r.Email,
		Avatar:    r.Avatar,
		Signature: r.Signature,
	}
}

type UpdateStatusRequest struct {
	Status int8 `json:"status"`
}
