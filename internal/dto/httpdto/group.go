package httpdto

import (
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/service"
)

type CreateGroupRequest struct {
	Name        string   `json:"name" binding:"required,max=50"`
	Notice      string   `json:"notice" binding:"max=500"`
	Avatar      string   `json:"avatar" binding:"max=255"`
	MemberUUIDs []string `json:"member_uuids"`
}

func (r CreateGroupRequest) ToInput() service.CreateGroupInput {
	return service.CreateGroupInput{
		Name:        r.Name,
		Notice:      r.Notice,
		Avatar:      r.Avatar,
		MemberUUIDs: r.MemberUUIDs,
	}
}

type AddGroupMembersRequest struct {
	MemberUUIDs []string `json:"member_uuids" binding:"required,min=1"`
}

type UpdateGroupRequest struct {
	Name   string `json:"name" binding:"omitempty,max=50"`
	Notice string `json:"notice" binding:"omitempty,max=500"`
	Avatar string `json:"avatar" binding:"omitempty,max=255"`
}

func (r UpdateGroupRequest) ToInput() service.UpdateGroupInput {
	return service.UpdateGroupInput{
		Name:   r.Name,
		Notice: r.Notice,
		Avatar: r.Avatar,
	}
}

type RemoveGroupMembersRequest struct {
	MemberUUIDs []string `json:"member_uuids" binding:"required,min=1"`
}

type GroupMemberResponse struct {
	User     *PublicUserResponse `json:"user"`
	Role     int8                `json:"role"`
	JoinedAt time.Time           `json:"joined_at"`
}

type GroupResponse struct {
	UUID        string                 `json:"uuid"`
	Name        string                 `json:"name"`
	Notice      string                 `json:"notice"`
	Avatar      string                 `json:"avatar"`
	Status      int8                   `json:"status"`
	MemberCount int                    `json:"member_count"`
	Owner       *PublicUserResponse    `json:"owner,omitempty"`
	MeRole      int8                   `json:"me_role"`
	Members     []*GroupMemberResponse `json:"members,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
}

func ToGroupResponse(view *service.GroupView) *GroupResponse {
	if view == nil || view.Group == nil {
		return nil
	}

	response := &GroupResponse{
		UUID:        view.Group.UUID,
		Name:        view.Group.Name,
		Notice:      view.Group.Notice,
		Avatar:      view.Group.Avatar,
		Status:      view.Group.Status,
		MemberCount: view.Group.MemberCount,
		Owner:       ToPublicUserResponse(view.Owner),
		MeRole:      view.MeRole,
		CreatedAt:   view.Group.CreatedAt,
	}
	if len(view.Members) > 0 {
		response.Members = ToGroupMemberResponses(view.Members)
	}

	return response
}

func ToGroupMemberResponses(items []*service.GroupMemberView) []*GroupMemberResponse {
	response := make([]*GroupMemberResponse, 0, len(items))
	for _, item := range items {
		if item == nil || item.Member == nil {
			continue
		}
		response = append(response, &GroupMemberResponse{
			User:     ToPublicUserResponse(item.User),
			Role:     item.Member.Role,
			JoinedAt: item.Member.JoinedAt,
		})
	}

	return response
}

func ToGroupMemberResponse(member *model.GroupMember, user *model.User) *GroupMemberResponse {
	if member == nil {
		return nil
	}

	return &GroupMemberResponse{
		User:     ToPublicUserResponse(user),
		Role:     member.Role,
		JoinedAt: member.JoinedAt,
	}
}
