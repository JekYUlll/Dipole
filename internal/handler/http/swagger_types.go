package http

import "github.com/JekYUlll/Dipole/internal/dto/httpdto"

// SuccessEnvelope 作为最通用的成功响应说明，适合只关心 envelope 结构的场景。
type SuccessEnvelope struct {
	Code int `json:"code"`
	Data any `json:"data"`
}

type ErrorEnvelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type AuthResponseEnvelope struct {
	Code int                   `json:"code"`
	Data *httpdto.AuthResponse `json:"data"`
}

type PublicUserResponseEnvelope struct {
	Code int                         `json:"code"`
	Data *httpdto.PublicUserResponse `json:"data"`
}

type PrivateUserResponseEnvelope struct {
	Code int                          `json:"code"`
	Data *httpdto.PrivateUserResponse `json:"data"`
}

type PublicUserListResponseEnvelope struct {
	Code int                           `json:"code"`
	Data []*httpdto.PublicUserResponse `json:"data"`
}

type PrivateUserListResponseEnvelope struct {
	Code int                            `json:"code"`
	Data []*httpdto.PrivateUserResponse `json:"data"`
}

type ContactListResponseEnvelope struct {
	Code int                        `json:"code"`
	Data []*httpdto.ContactResponse `json:"data"`
}

type ContactApplicationListResponseEnvelope struct {
	Code int                                   `json:"code"`
	Data []*httpdto.ContactApplicationResponse `json:"data"`
}

type ConversationListResponseEnvelope struct {
	Code int                             `json:"code"`
	Data []*httpdto.ConversationResponse `json:"data"`
}

type GroupResponseEnvelope struct {
	Code int                    `json:"code"`
	Data *httpdto.GroupResponse `json:"data"`
}

type GroupMemberListResponseEnvelope struct {
	Code int                            `json:"code"`
	Data []*httpdto.GroupMemberResponse `json:"data"`
}

type MessageListResponseEnvelope struct {
	Code int                        `json:"code"`
	Data []*httpdto.MessageResponse `json:"data"`
}

type UploadedFileResponseEnvelope struct {
	Code int                           `json:"code"`
	Data *httpdto.UploadedFileResponse `json:"data"`
}

type FileDownloadResponseEnvelope struct {
	Code int                           `json:"code"`
	Data *httpdto.FileDownloadResponse `json:"data"`
}

type FileMultipartInitiateResponseEnvelope struct {
	Code int                                    `json:"code"`
	Data *httpdto.FileMultipartInitiateResponse `json:"data"`
}

type DeviceSessionListResponseEnvelope struct {
	Code int                              `json:"code"`
	Data []*httpdto.DeviceSessionResponse `json:"data"`
}

type AdminOverviewResponseEnvelope struct {
	Code int                            `json:"code"`
	Data *httpdto.AdminOverviewResponse `json:"data"`
}

type MessageOnlyResponse struct {
	Message string `json:"message"`
}

type MessageOnlyResponseEnvelope struct {
	Code int                 `json:"code"`
	Data MessageOnlyResponse `json:"data"`
}

type IDStatusResponse struct {
	ID        uint   `json:"id,omitempty"`
	Status    int8   `json:"status,omitempty"`
	Message   string `json:"message,omitempty"`
	HandledAt any    `json:"handled_at,omitempty"`
}

type IDStatusResponseEnvelope struct {
	Code int              `json:"code"`
	Data IDStatusResponse `json:"data"`
}

type ContactRemarkResponse struct {
	FriendUUID string `json:"friend_uuid"`
	Remark     string `json:"remark"`
	Status     int8   `json:"status"`
}

type ContactRemarkResponseEnvelope struct {
	Code int                   `json:"code"`
	Data ContactRemarkResponse `json:"data"`
}

type ContactBlockResponse struct {
	FriendUUID string `json:"friend_uuid"`
	Blocked    bool   `json:"blocked"`
	Status     int8   `json:"status"`
}

type ContactBlockResponseEnvelope struct {
	Code int                  `json:"code"`
	Data ContactBlockResponse `json:"data"`
}

type ConversationRemarkResponse struct {
	ConversationKey string `json:"conversation_key"`
	Remark          string `json:"remark"`
}

type ConversationRemarkResponseEnvelope struct {
	Code int                        `json:"code"`
	Data ConversationRemarkResponse `json:"data"`
}

type MultipartPartResponse struct {
	PartNumber int `json:"part_number"`
}

type MultipartPartResponseEnvelope struct {
	Code int                   `json:"code"`
	Data MultipartPartResponse `json:"data"`
}

type MultipartAbortResponse struct {
	Aborted bool `json:"aborted"`
}

type MultipartAbortResponseEnvelope struct {
	Code int                    `json:"code"`
	Data MultipartAbortResponse `json:"data"`
}

type DeviceLogoutResponse struct {
	Message      string `json:"message"`
	ConnectionID string `json:"connection_id,omitempty"`
}

type DeviceLogoutResponseEnvelope struct {
	Code int                  `json:"code"`
	Data DeviceLogoutResponse `json:"data"`
}
