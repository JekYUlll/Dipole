package coregrpc

import (
	"context"
	"errors"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	grpccommon "github.com/JekYUlll/Dipole/internal/transport/grpc/common"
	corev1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/core/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	corev1.UnimplementedCoreCapabilityServiceServer
	capability application.CoreCapability
}

func NewServer(capability application.CoreCapability) (*Server, error) {
	if capability == nil {
		return nil, errors.New("core capability is required")
	}
	return &Server{capability: capability}, nil
}

func (s *Server) GetUser(_ context.Context, request *corev1.GetUserRequest) (*corev1.GetUserResponse, error) {
	if _, err := grpccommon.Caller(request.GetContext()); err != nil {
		return nil, err
	}
	user, err := s.capability.GetUserByUUID(request.GetUserId())
	if err != nil {
		return nil, status.Error(codes.Internal, "core user lookup failed")
	}
	return &corev1.GetUserResponse{User: userToProto(user)}, nil
}

func (s *Server) CanSendDirectMessage(_ context.Context, request *corev1.CanSendDirectMessageRequest) (*corev1.CanSendDirectMessageResponse, error) {
	if _, err := grpccommon.Caller(request.GetContext()); err != nil {
		return nil, err
	}
	allowed, err := s.capability.CanSendDirectMessage(request.GetUserId(), request.GetTargetUserId())
	if err != nil {
		return nil, status.Error(codes.Internal, "core direct authorization failed")
	}
	return &corev1.CanSendDirectMessageResponse{Allowed: allowed}, nil
}

func (s *Server) GetGroup(_ context.Context, request *corev1.GetGroupRequest) (*corev1.GetGroupResponse, error) {
	if _, err := grpccommon.Caller(request.GetContext()); err != nil {
		return nil, err
	}
	group, err := s.capability.GetGroupByUUID(request.GetGroupId())
	if err != nil {
		return nil, status.Error(codes.Internal, "core group lookup failed")
	}
	return &corev1.GetGroupResponse{Group: groupToProto(group)}, nil
}

func (s *Server) GetGroupMember(_ context.Context, request *corev1.GetGroupMemberRequest) (*corev1.GetGroupMemberResponse, error) {
	if _, err := grpccommon.Caller(request.GetContext()); err != nil {
		return nil, err
	}
	member, err := s.capability.GetGroupMember(request.GetGroupId(), request.GetUserId())
	if err != nil {
		return nil, status.Error(codes.Internal, "core group member lookup failed")
	}
	return &corev1.GetGroupMemberResponse{Member: memberToProto(member)}, nil
}

func (s *Server) ListGroupMembers(_ context.Context, request *corev1.ListGroupMembersRequest) (*corev1.ListGroupMembersResponse, error) {
	if _, err := grpccommon.Caller(request.GetContext()); err != nil {
		return nil, err
	}
	members, err := s.capability.ListGroupMembers(request.GetGroupId())
	if err != nil {
		return nil, status.Error(codes.Internal, "core group member list failed")
	}
	response := &corev1.ListGroupMembersResponse{Members: make([]*corev1.GroupMemberSnapshot, 0, len(members))}
	for _, member := range members {
		if member != nil {
			response.Members = append(response.Members, memberToProto(member))
		}
	}
	return response, nil
}

func (s *Server) GetOwnedFile(_ context.Context, request *corev1.GetOwnedFileRequest) (*corev1.GetOwnedFileResponse, error) {
	if _, err := grpccommon.Caller(request.GetContext()); err != nil {
		return nil, err
	}
	file, err := s.capability.GetOwnedFile(request.GetUploaderUserId(), request.GetFileId())
	if err != nil {
		return nil, status.Error(codes.Internal, "core file lookup failed")
	}
	return &corev1.GetOwnedFileResponse{File: fileToProto(file)}, nil
}

func userToProto(user *model.User) *corev1.UserSnapshot {
	if user == nil {
		return nil
	}
	return &corev1.UserSnapshot{
		UserId:   user.UUID,
		Nickname: user.Nickname,
		Avatar:   user.Avatar,
		UserType: int32(user.UserType),
		Status:   int32(user.Status),
		IsAdmin:  user.IsAdmin,
	}
}

func groupToProto(group *model.Group) *corev1.GroupSnapshot {
	if group == nil {
		return nil
	}
	return &corev1.GroupSnapshot{
		GroupId:     group.UUID,
		Name:        group.Name,
		OwnerUserId: group.OwnerUUID,
		MemberCount: int32(group.MemberCount),
		Status:      int32(group.Status),
	}
}

func memberToProto(member *model.GroupMember) *corev1.GroupMemberSnapshot {
	if member == nil {
		return nil
	}
	return &corev1.GroupMemberSnapshot{GroupId: member.GroupUUID, UserId: member.UserUUID, Role: int32(member.Role)}
}

func fileToProto(file *model.UploadedFile) *corev1.FileSnapshot {
	if file == nil {
		return nil
	}
	return &corev1.FileSnapshot{
		FileId:         file.UUID,
		UploaderUserId: file.UploaderUUID,
		FileName:       file.FileName,
		FileSize:       file.FileSize,
		ContentType:    file.ContentType,
		Url:            file.URL,
	}
}
