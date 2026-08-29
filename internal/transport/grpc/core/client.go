package coregrpc

import (
	"context"
	"errors"
	"strings"
	"time"

	commonv1 "github.com/JekYUlll/Dipole/api/gen/go/common/v1"
	corev1 "github.com/JekYUlll/Dipole/api/gen/go/core/v1"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	grpccommon "github.com/JekYUlll/Dipole/internal/transport/grpc/common"
)

const queryTimeout = 2 * time.Second

type Client struct {
	rpc           corev1.CoreCapabilityServiceClient
	callerService string
}

var _ application.CoreCapability = (*Client)(nil)

func NewClient(rpc corev1.CoreCapabilityServiceClient) (*Client, error) {
	return NewClientForService(rpc, "dipole-message")
}

func NewClientForService(rpc corev1.CoreCapabilityServiceClient, callerService string) (*Client, error) {
	if rpc == nil {
		return nil, errors.New("core capability rpc client is required")
	}
	callerService = strings.TrimSpace(callerService)
	if callerService == "" {
		return nil, errors.New("core capability caller service is required")
	}
	return &Client{rpc: rpc, callerService: callerService}, nil
}

func (c *Client) GetUserByUUID(userUUID string) (*model.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	response, err := c.rpc.GetUser(ctx, &corev1.GetUserRequest{Context: c.requestContext(""), UserId: userUUID})
	if err != nil {
		return nil, err
	}
	return userFromProto(response.GetUser()), nil
}

func (c *Client) CanSendDirectMessage(userUUID, friendUUID string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	response, err := c.rpc.CanSendDirectMessage(ctx, &corev1.CanSendDirectMessageRequest{
		Context:      c.requestContext(userUUID),
		UserId:       userUUID,
		TargetUserId: friendUUID,
	})
	if err != nil {
		return false, err
	}
	return response.GetAllowed(), nil
}

func (c *Client) GetGroupByUUID(groupUUID string) (*model.Group, error) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	response, err := c.rpc.GetGroup(ctx, &corev1.GetGroupRequest{Context: c.requestContext(""), GroupId: groupUUID})
	if err != nil {
		return nil, err
	}
	return groupFromProto(response.GetGroup()), nil
}

func (c *Client) GetGroupMember(groupUUID, userUUID string) (*model.GroupMember, error) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	response, err := c.rpc.GetGroupMember(ctx, &corev1.GetGroupMemberRequest{
		Context: c.requestContext(userUUID),
		GroupId: groupUUID,
		UserId:  userUUID,
	})
	if err != nil {
		return nil, err
	}
	return memberFromProto(response.GetMember()), nil
}

func (c *Client) ListGroupMembers(groupUUID string) ([]*model.GroupMember, error) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	response, err := c.rpc.ListGroupMembers(ctx, &corev1.ListGroupMembersRequest{Context: c.requestContext(""), GroupId: groupUUID})
	if err != nil {
		return nil, err
	}
	members := make([]*model.GroupMember, 0, len(response.GetMembers()))
	for _, member := range response.GetMembers() {
		if member != nil {
			members = append(members, memberFromProto(member))
		}
	}
	return members, nil
}

func (c *Client) GetOwnedFile(uploaderUUID, fileUUID string) (*model.UploadedFile, error) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	response, err := c.rpc.GetOwnedFile(ctx, &corev1.GetOwnedFileRequest{
		Context:        c.requestContext(uploaderUUID),
		UploaderUserId: uploaderUUID,
		FileId:         fileUUID,
	})
	if err != nil {
		return nil, err
	}
	return fileFromProto(response.GetFile()), nil
}

func (c *Client) ListSearchConversationKeys(userUUID string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	response, err := c.rpc.ListSearchConversationKeys(ctx, &corev1.ListSearchConversationKeysRequest{
		Context: c.requestContext(userUUID),
	})
	if err != nil {
		return nil, err
	}
	return append([]string(nil), response.GetConversationKeys()...), nil
}

func (c *Client) requestContext(principal string) *commonv1.RequestContext {
	return grpccommon.RequestContext(principal, c.callerService)
}

func userFromProto(user *corev1.UserSnapshot) *model.User {
	if user == nil {
		return nil
	}
	return &model.User{UUID: user.GetUserId(), Nickname: user.GetNickname(), Avatar: user.GetAvatar(), UserType: int8(user.GetUserType()), Status: int8(user.GetStatus()), IsAdmin: user.GetIsAdmin()}
}

func groupFromProto(group *corev1.GroupSnapshot) *model.Group {
	if group == nil {
		return nil
	}
	return &model.Group{UUID: group.GetGroupId(), Name: group.GetName(), OwnerUUID: group.GetOwnerUserId(), MemberCount: int(group.GetMemberCount()), Status: int8(group.GetStatus())}
}

func memberFromProto(member *corev1.GroupMemberSnapshot) *model.GroupMember {
	if member == nil {
		return nil
	}
	return &model.GroupMember{GroupUUID: member.GetGroupId(), UserUUID: member.GetUserId(), Role: int8(member.GetRole())}
}

func fileFromProto(file *corev1.FileSnapshot) *model.UploadedFile {
	if file == nil {
		return nil
	}
	return &model.UploadedFile{
		UUID:         file.GetFileId(),
		UploaderUUID: file.GetUploaderUserId(),
		FileName:     file.GetFileName(),
		FileSize:     file.GetFileSize(),
		ContentType:  file.GetContentType(),
		URL:          file.GetUrl(),
	}
}
