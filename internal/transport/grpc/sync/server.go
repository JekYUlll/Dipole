package syncgrpc

import (
	"context"
	"errors"

	"github.com/JekYUlll/Dipole/internal/application"
	grpccommon "github.com/JekYUlll/Dipole/internal/transport/grpc/common"
	syncv1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/sync/v1"
	grpcmapping "github.com/JekYUlll/Dipole/internal/transport/grpc/mapping"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	syncv1.UnimplementedSyncQueryServiceServer
	application application.SyncApplication
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
