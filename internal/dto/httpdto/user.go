package httpdto

import coreuser "github.com/JekYUlll/Dipole/internal/services/core/domain/user"

type UpdateProfileRequest struct {
	Nickname  *string `json:"nickname"`
	Email     *string `json:"email"`
	Avatar    *string `json:"avatar"`
	Signature *string `json:"signature"`
}

func (r UpdateProfileRequest) ToInput() coreuser.UpdateProfileInput {
	return coreuser.UpdateProfileInput{
		Nickname:  r.Nickname,
		Email:     r.Email,
		Avatar:    r.Avatar,
		Signature: r.Signature,
	}
}

type UpdateStatusRequest struct {
	Status int8 `json:"status"`
}
