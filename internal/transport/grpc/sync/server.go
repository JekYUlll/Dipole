package syncgrpc

import (
	"context"
	"errors"
	"strings"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/service"
	grpccommon "github.com/JekYUlll/Dipole/internal/transport/grpc/common"
	commonv1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/common/v1"
	syncv1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/sync/v1"
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

func syncCheckpointStatus(err error) error {
	if errors.Is(err, service.ErrSyncDeviceIDRequired) || errors.Is(err, service.ErrSyncDeviceIDInvalid) || errors.Is(err, service.ErrSyncCheckpointAhead) {
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
		response.Items = append(response.Items, &syncv1.SyncMessage{
			SyncSeq:         item.SyncSeq,
			ConversationKey: item.ConversationKey,
			Message:         grpcmapping.MessageToProto(item.Message),
		})
	}
	return response, nil
}
