package searchgrpc

import (
	"context"
	"errors"
	"fmt"

	searchv1 "github.com/JekYUlll/Dipole/api/gen/go/search/v1"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	grpccommon "github.com/JekYUlll/Dipole/internal/transport/grpc/common"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	searchv1.UnimplementedSearchServiceServer
	application application.SearchApplication
}

func NewServer(application application.SearchApplication) (*Server, error) {
	if application == nil {
		return nil, errors.New("Search application is required")
	}
	return &Server{application: application}, nil
}

func (s *Server) SearchMessages(ctx context.Context, request *searchv1.SearchMessagesRequest) (*searchv1.SearchMessagesResponse, error) {
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
	documents, err := s.application.Search(principal, request.GetQuery(), int(request.GetPageSize()))
	if err != nil {
		if errors.Is(err, application.ErrSearchTextRequired) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, "search service failed")
	}
	response := &searchv1.SearchMessagesResponse{Messages: make([]*searchv1.SearchMessage, 0, len(documents))}
	for _, document := range documents {
		if document == nil {
			continue
		}
		message, err := searchMessageToProto(document)
		if err != nil {
			return nil, status.Error(codes.Internal, "search service returned an invalid document")
		}
		response.Messages = append(response.Messages, message)
	}
	return response, nil
}

func searchMessageToProto(document *model.MessageSearchDocument) (*searchv1.SearchMessage, error) {
	sentAt := timestamppb.New(document.SentAt)
	if err := sentAt.CheckValid(); err != nil {
		return nil, fmt.Errorf("Search document sent_at: %w", err)
	}
	return &searchv1.SearchMessage{
		ServerMessageId: document.MessageUUID,
		ConversationKey: document.ConversationKey,
		Sequence:        document.MessageSeq,
		Revision:        document.Revision,
		SenderId:        document.SenderUUID,
		MessageType:     int32(document.MessageType),
		Content:         document.Content,
		SentAt:          sentAt,
	}, nil
}
