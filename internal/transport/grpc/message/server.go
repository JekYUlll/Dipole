package messagegrpc

import (
	"context"
	"errors"
	"strings"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	messagev1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/message/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	messagev1.UnimplementedMessageServiceServer
	application application.MessageApplication
}

func NewServer(application application.MessageApplication) (*Server, error) {
	if application == nil {
		return nil, errors.New("message application is required")
	}
	return &Server{application: application}, nil
}

func (s *Server) SendDirectText(_ context.Context, request *messagev1.SendDirectTextRequest) (*messagev1.SendMessageResponse, error) {
	principal, err := principalFrom(request.GetContext())
	if err != nil {
		return nil, err
	}
	message, err := s.application.SendDirectMessage(principal, request.GetTargetUserId(), request.GetContent(), request.GetClientMessageId())
	if err != nil {
		return nil, rpcError(err)
	}
	return sendResponse(message, nil), nil
}

func (s *Server) SendGroupText(_ context.Context, request *messagev1.SendGroupTextRequest) (*messagev1.SendMessageResponse, error) {
	principal, err := principalFrom(request.GetContext())
	if err != nil {
		return nil, err
	}
	message, recipients, err := s.application.SendGroupMessage(principal, request.GetGroupId(), request.GetContent(), request.GetClientMessageId())
	if err != nil {
		return nil, rpcError(err)
	}
	return sendResponse(message, recipients), nil
}

func (s *Server) SendDirectFile(_ context.Context, request *messagev1.SendDirectFileRequest) (*messagev1.SendMessageResponse, error) {
	principal, err := principalFrom(request.GetContext())
	if err != nil {
		return nil, err
	}
	message, err := s.application.SendDirectFileMessage(principal, request.GetTargetUserId(), request.GetFileId(), request.GetClientMessageId())
	if err != nil {
		return nil, rpcError(err)
	}
	return sendResponse(message, nil), nil
}

func (s *Server) SendGroupFile(_ context.Context, request *messagev1.SendGroupFileRequest) (*messagev1.SendMessageResponse, error) {
	principal, err := principalFrom(request.GetContext())
	if err != nil {
		return nil, err
	}
	message, recipients, err := s.application.SendGroupFileMessage(principal, request.GetGroupId(), request.GetFileId(), request.GetClientMessageId())
	if err != nil {
		return nil, rpcError(err)
	}
	return sendResponse(message, recipients), nil
}

func (s *Server) ListDirectHistory(_ context.Context, request *messagev1.ListDirectHistoryRequest) (*messagev1.ListMessagesResponse, error) {
	principal, err := principalFrom(request.GetContext())
	if err != nil {
		return nil, err
	}
	beforeID, err := uintCursor(request.GetBeforeId())
	if err != nil {
		return nil, err
	}
	pageSize, err := pageSize(request.GetPageSize())
	if err != nil {
		return nil, err
	}
	messages, appErr := s.application.ListDirectMessages(principal, request.GetTargetUserId(), beforeID, pageSize)
	if appErr != nil {
		return nil, rpcError(appErr)
	}
	return listResponse(messages), nil
}

func (s *Server) ListGroupHistory(_ context.Context, request *messagev1.ListGroupHistoryRequest) (*messagev1.ListMessagesResponse, error) {
	principal, err := principalFrom(request.GetContext())
	if err != nil {
		return nil, err
	}
	limit, err := pageSize(request.GetPageSize())
	if err != nil {
		return nil, err
	}

	var messages []*model.Message
	switch cursor := request.GetCursor().(type) {
	case *messagev1.ListGroupHistoryRequest_AfterId:
		afterID, cursorErr := uintCursor(cursor.AfterId)
		if cursorErr != nil {
			return nil, cursorErr
		}
		messages, err = s.application.ListGroupMessagesAfter(principal, request.GetGroupId(), afterID, limit)
	case *messagev1.ListGroupHistoryRequest_BeforeId:
		beforeID, cursorErr := uintCursor(cursor.BeforeId)
		if cursorErr != nil {
			return nil, cursorErr
		}
		messages, err = s.application.ListGroupMessages(principal, request.GetGroupId(), beforeID, limit)
	default:
		messages, err = s.application.ListGroupMessages(principal, request.GetGroupId(), 0, limit)
	}
	if err != nil {
		return nil, rpcError(err)
	}
	return listResponse(messages), nil
}

func (s *Server) ListOfflineMessages(_ context.Context, request *messagev1.ListOfflineMessagesRequest) (*messagev1.ListMessagesResponse, error) {
	principal, err := principalFrom(request.GetContext())
	if err != nil {
		return nil, err
	}
	afterID, err := uintCursor(request.GetAfterId())
	if err != nil {
		return nil, err
	}
	limit, err := pageSize(request.GetPageSize())
	if err != nil {
		return nil, err
	}
	messages, appErr := s.application.ListOfflineMessages(principal, afterID, limit)
	if appErr != nil {
		return nil, rpcError(appErr)
	}
	return listResponse(messages), nil
}

func principalFrom(invocation *messagev1.InvocationContext) (string, error) {
	if invocation == nil || strings.TrimSpace(invocation.GetPrincipalUserId()) == "" {
		return "", status.Error(codes.Unauthenticated, "principal_user_id is required")
	}
	return strings.TrimSpace(invocation.GetPrincipalUserId()), nil
}

func uintCursor(value uint64) (uint, error) {
	if uint64(uint(value)) != value {
		return 0, status.Error(codes.InvalidArgument, "cursor exceeds server range")
	}
	return uint(value), nil
}

func pageSize(value int32) (int, error) {
	if value < 0 {
		return 0, status.Error(codes.InvalidArgument, "page_size cannot be negative")
	}
	return int(value), nil
}

func rpcError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, application.ErrMessageTargetRequired):
		return statusWithReason(codes.InvalidArgument, err, messagev1.ErrorReason_ERROR_REASON_TARGET_REQUIRED)
	case errors.Is(err, application.ErrMessageContentRequired):
		return statusWithReason(codes.InvalidArgument, err, messagev1.ErrorReason_ERROR_REASON_CONTENT_REQUIRED)
	case errors.Is(err, application.ErrMessageContentTooLong):
		return statusWithReason(codes.InvalidArgument, err, messagev1.ErrorReason_ERROR_REASON_CONTENT_TOO_LONG)
	case errors.Is(err, application.ErrMessageFileRequired):
		return statusWithReason(codes.InvalidArgument, err, messagev1.ErrorReason_ERROR_REASON_FILE_REQUIRED)
	case errors.Is(err, application.ErrMessageTargetNotFound):
		return statusWithReason(codes.NotFound, err, messagev1.ErrorReason_ERROR_REASON_TARGET_NOT_FOUND)
	case errors.Is(err, application.ErrMessageFriendRequired):
		return statusWithReason(codes.PermissionDenied, err, messagev1.ErrorReason_ERROR_REASON_FRIEND_REQUIRED)
	case errors.Is(err, application.ErrMessageGroupForbidden):
		return statusWithReason(codes.PermissionDenied, err, messagev1.ErrorReason_ERROR_REASON_GROUP_FORBIDDEN)
	case errors.Is(err, application.ErrMessageIdempotencyConflict):
		return statusWithReason(codes.AlreadyExists, err, messagev1.ErrorReason_ERROR_REASON_IDEMPOTENCY_CONFLICT)
	case errors.Is(err, application.ErrMessageTargetUnavailable):
		return statusWithReason(codes.FailedPrecondition, err, messagev1.ErrorReason_ERROR_REASON_TARGET_UNAVAILABLE)
	case errors.Is(err, application.ErrMessageFileUnavailable):
		return statusWithReason(codes.FailedPrecondition, err, messagev1.ErrorReason_ERROR_REASON_FILE_UNAVAILABLE)
	default:
		return status.Error(codes.Internal, "message service failed")
	}
}

func statusWithReason(code codes.Code, err error, reason messagev1.ErrorReason) error {
	base := status.New(code, err.Error())
	withDetails, detailErr := base.WithDetails(&messagev1.ErrorDetail{Reason: reason})
	if detailErr != nil {
		return base.Err()
	}
	return withDetails.Err()
}

func sendResponse(message *model.Message, recipients []string) *messagev1.SendMessageResponse {
	return &messagev1.SendMessageResponse{
		Message:          messageToProto(message),
		RecipientUserIds: recipients,
	}
}

func listResponse(messages []*model.Message) *messagev1.ListMessagesResponse {
	response := &messagev1.ListMessagesResponse{Messages: make([]*messagev1.Message, 0, len(messages))}
	for _, message := range messages {
		if message == nil {
			continue
		}
		response.Messages = append(response.Messages, messageToProto(message))
		if response.FirstId == 0 {
			response.FirstId = uint64(message.ID)
		}
		response.LastId = uint64(message.ID)
	}
	return response
}

func messageToProto(message *model.Message) *messagev1.Message {
	if message == nil {
		return nil
	}
	result := &messagev1.Message{
		Id:              uint64(message.ID),
		ServerMessageId: message.UUID,
		ClientMessageId: message.ClientMessageID,
		ConversationKey: message.ConversationKey,
		SenderId:        message.SenderUUID,
		TargetType:      int32(message.TargetType),
		TargetId:        message.TargetUUID,
		MessageType:     int32(message.MessageType),
		Content:         message.Content,
		FileId:          message.FileID,
		FileName:        message.FileName,
		FileSize:        message.FileSize,
		FileUrl:         message.FileURL,
		FileContentType: message.FileContentType,
		SentAt:          timestamppb.New(message.SentAt),
	}
	if message.FileExpiresAt != nil {
		result.FileExpiresAt = timestamppb.New(*message.FileExpiresAt)
	}
	return result
}
