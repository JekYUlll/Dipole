package syncgrpc

import (
	"context"
	"errors"
	"strings"

	commonv1 "github.com/JekYUlll/Dipole/api/gen/go/common/v1"
	syncv1 "github.com/JekYUlll/Dipole/api/gen/go/sync/v1"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/compat/service"
	"github.com/JekYUlll/Dipole/internal/model"
	grpccommon "github.com/JekYUlll/Dipole/internal/transport/grpc/common"
	grpcmapping "github.com/JekYUlll/Dipole/internal/transport/grpc/mapping"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	syncv1.UnimplementedSyncQueryServiceServer
	application application.SyncApplication
}

func (s *Server) GetDeviceCheckpoint(ctx context.Context, request *syncv1.GetDeviceCheckpointRequest) (*syncv1.DeviceCheckpointResponse, error) {
	principal, deviceID, err := syncPrincipalAndDevice(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	checkpoint, err := s.application.GetCheckpoint(principal, deviceID)
	if err != nil {
		return nil, syncCheckpointStatus(err)
	}
	return checkpointResponse(checkpoint), nil
}

func (s *Server) AdvanceDeviceCheckpoint(ctx context.Context, request *syncv1.AdvanceDeviceCheckpointRequest) (*syncv1.DeviceCheckpointResponse, error) {
	principal, deviceID, err := syncPrincipalAndDevice(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	checkpoint, err := s.application.AdvanceCheckpoint(principal, deviceID, request.GetSyncSeq())
	if err != nil {
		return nil, syncCheckpointStatus(err)
	}
	return checkpointResponse(checkpoint), nil
}

func (s *Server) ListGroupCheckpoints(ctx context.Context, request *syncv1.ListGroupCheckpointsRequest) (*syncv1.ListGroupCheckpointsResponse, error) {
	principal, deviceID, err := syncPrincipalAndDevice(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	checkpoints, err := s.application.ListGroupCheckpoints(principal, deviceID, request.GetGroupIds())
	if err != nil {
		return nil, syncCheckpointStatus(err)
	}
	response := &syncv1.ListGroupCheckpointsResponse{Checkpoints: make([]*syncv1.GroupCheckpoint, 0, len(checkpoints))}
	for _, checkpoint := range checkpoints {
		response.Checkpoints = append(response.Checkpoints, groupCheckpointResponse(checkpoint))
	}
	return response, nil
}

func (s *Server) AdvanceGroupCheckpoint(ctx context.Context, request *syncv1.AdvanceGroupCheckpointRequest) (*syncv1.GroupCheckpoint, error) {
	principal, deviceID, err := syncPrincipalAndDevice(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	checkpoint, err := s.application.AdvanceGroupCheckpoint(principal, deviceID, request.GetGroupId(), request.GetMessageSequence())
	if err != nil {
		return nil, syncCheckpointStatus(err)
	}
	return groupCheckpointResponse(checkpoint), nil
}

func syncPrincipalAndDevice(ctx context.Context, requestContext *commonv1.RequestContext) (string, string, error) {
	if _, err := grpccommon.Caller(ctx, requestContext); err != nil {
		return "", "", err
	}
	principal, err := grpccommon.Principal(requestContext)
	if err != nil {
		return "", "", err
	}
	deviceID := strings.TrimSpace(requestContext.GetDeviceId())
	if deviceID == "" {
		return "", "", status.Error(codes.InvalidArgument, "device_id is required")
	}
	return principal, deviceID, nil
}

func checkpointResponse(checkpoint *model.DeviceSyncCheckpoint) *syncv1.DeviceCheckpointResponse {
	if checkpoint == nil {
		return &syncv1.DeviceCheckpointResponse{}
	}
	return &syncv1.DeviceCheckpointResponse{DeviceId: checkpoint.DeviceID, SyncSeq: checkpoint.SyncSeq}
}

func groupCheckpointResponse(checkpoint *model.GroupSyncCheckpoint) *syncv1.GroupCheckpoint {
	if checkpoint == nil {
		return &syncv1.GroupCheckpoint{}
	}
	return &syncv1.GroupCheckpoint{GroupId: checkpoint.GroupUUID, LatestMessageSequence: checkpoint.LatestMessageSeq, LatestMessageId: checkpoint.LatestMessageUUID, PulledMessageSequence: checkpoint.PulledMessageSeq}
}

func syncCheckpointStatus(err error) error {
	if errors.Is(err, service.ErrSyncGroupForbidden) {
		return status.Error(codes.PermissionDenied, err.Error())
	}
	if errors.Is(err, service.ErrSyncDeviceIDRequired) || errors.Is(err, service.ErrSyncDeviceIDInvalid) || errors.Is(err, service.ErrSyncCheckpointAhead) {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	if errors.Is(err, service.ErrSyncGroupRequired) || errors.Is(err, service.ErrSyncGroupLimit) {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	return status.Error(codes.Internal, "sync checkpoint operation failed")
}

func NewServer(application application.SyncApplication) (*Server, error) {
	if application == nil {
		return nil, errors.New("sync application is required")
	}
	return &Server{application: application}, nil
}

func (s *Server) ListSyncMessages(ctx context.Context, request *syncv1.ListSyncMessagesRequest) (*syncv1.ListSyncMessagesResponse, error) {
	if _, err := grpccommon.Caller(ctx, request.GetContext()); err != nil {
		return nil, err
	}
	principal, err := grpccommon.Principal(request.GetContext())
	if err != nil {
		return nil, err
	}
	if request.GetPageSize() < 0 {
		return nil, status.Error(codes.InvalidArgument, "page_size cannot be negative")
	}
	page, err := s.application.List(principal, request.GetAfterSeq(), int(request.GetPageSize()))
	if err != nil {
		return nil, status.Error(codes.Internal, "sync query failed")
	}
	response := &syncv1.ListSyncMessagesResponse{NextSeq: request.GetAfterSeq()}
	if page == nil {
		return response, nil
	}
	response.NextSeq = page.NextSeq
	response.HasMore = page.HasMore
	response.Items = make([]*syncv1.SyncMessage, 0, len(page.Items))
	for _, item := range page.Items {
		if item == nil {
			continue
		}
		messageID, messageSequence := item.MessageUUID, item.MessageSeq
		if item.Message != nil {
			if messageID == "" {
				messageID = item.Message.UUID
			}
			if messageSequence == 0 {
				messageSequence = item.Message.Seq
			}
		}
		response.Items = append(response.Items, &syncv1.SyncMessage{
			SyncSeq:         item.SyncSeq,
			ConversationKey: item.ConversationKey,
			Message:         grpcmapping.MessageToProto(item.Message),
			MessageUuid:     messageID,
			MessageSeq:      messageSequence,
		})
	}
	return response, nil
}
