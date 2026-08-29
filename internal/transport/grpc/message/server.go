package messagegrpc

import (
	"context"
	"errors"
	"strings"

	commonv1 "github.com/JekYUlll/Dipole/api/gen/go/common/v1"
	messagev1 "github.com/JekYUlll/Dipole/api/gen/go/message/v1"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	grpcauth "github.com/JekYUlll/Dipole/internal/transport/grpc/auth"
	grpccommon "github.com/JekYUlll/Dipole/internal/transport/grpc/common"
	grpcmapping "github.com/JekYUlll/Dipole/internal/transport/grpc/mapping"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

func (s *Server) SendDirectText(ctx context.Context, request *messagev1.SendDirectTextRequest) (*messagev1.SendMessageResponse, error) {
	ctx = grpccommon.Correlation(ctx, request.GetContext())
	principal, err := principalFrom(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	var message *model.Message
	if contextual, ok := s.application.(application.MessageCommandContext); ok {
		message, err = contextual.SendDirectMessageContext(ctx, principal, request.GetTargetUserId(), request.GetContent(), request.GetClientMessageId())
	} else {
		message, err = s.application.SendDirectMessage(principal, request.GetTargetUserId(), request.GetContent(), request.GetClientMessageId())
	}
	if err != nil {
		return nil, rpcError(err)
	}
	return sendResponse(message, nil), nil
}

func (s *Server) SendGroupText(ctx context.Context, request *messagev1.SendGroupTextRequest) (*messagev1.SendMessageResponse, error) {
	ctx = grpccommon.Correlation(ctx, request.GetContext())
	principal, err := principalFrom(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	var message *model.Message
	var recipients []string
	if contextual, ok := s.application.(application.MessageCommandContext); ok {
		message, recipients, err = contextual.SendGroupMessageContext(ctx, principal, request.GetGroupId(), request.GetContent(), request.GetClientMessageId())
	} else {
		message, recipients, err = s.application.SendGroupMessage(principal, request.GetGroupId(), request.GetContent(), request.GetClientMessageId())
	}
	if err != nil {
		return nil, rpcError(err)
	}
	return sendResponse(message, recipients), nil
}

func (s *Server) SendDirectFile(ctx context.Context, request *messagev1.SendDirectFileRequest) (*messagev1.SendMessageResponse, error) {
	ctx = grpccommon.Correlation(ctx, request.GetContext())
	principal, err := principalFrom(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	var message *model.Message
	if contextual, ok := s.application.(application.MessageCommandContext); ok {
		message, err = contextual.SendDirectFileMessageContext(ctx, principal, request.GetTargetUserId(), request.GetFileId(), request.GetClientMessageId())
	} else {
		message, err = s.application.SendDirectFileMessage(principal, request.GetTargetUserId(), request.GetFileId(), request.GetClientMessageId())
	}
	if err != nil {
		return nil, rpcError(err)
	}
	return sendResponse(message, nil), nil
}

func (s *Server) SendGroupFile(ctx context.Context, request *messagev1.SendGroupFileRequest) (*messagev1.SendMessageResponse, error) {
	ctx = grpccommon.Correlation(ctx, request.GetContext())
	principal, err := principalFrom(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	var message *model.Message
	var recipients []string
	if contextual, ok := s.application.(application.MessageCommandContext); ok {
		message, recipients, err = contextual.SendGroupFileMessageContext(ctx, principal, request.GetGroupId(), request.GetFileId(), request.GetClientMessageId())
	} else {
		message, recipients, err = s.application.SendGroupFileMessage(principal, request.GetGroupId(), request.GetFileId(), request.GetClientMessageId())
	}
	if err != nil {
		return nil, rpcError(err)
	}
	return sendResponse(message, recipients), nil
}

func (s *Server) SendSystemDirectMessage(ctx context.Context, request *messagev1.SendSystemDirectMessageRequest) (*messagev1.SendMessageResponse, error) {
	if err := requireCoreCaller(ctx, request.GetContext()); err != nil {
		return nil, err
	}
	sender, ok := s.application.(application.SystemMessageSender)
	if !ok {
		return nil, status.Error(codes.Unimplemented, "system message sender is unavailable")
	}
	message, err := sender.SendSystemDirectMessage(request.GetSenderUserId(), request.GetTargetUserId(), request.GetContent())
	if err != nil {
		return nil, rpcError(err)
	}
	return sendResponse(message, nil), nil
}

func (s *Server) SendSystemGroupMessage(ctx context.Context, request *messagev1.SendSystemGroupMessageRequest) (*messagev1.SendMessageResponse, error) {
	if err := requireCoreCaller(ctx, request.GetContext()); err != nil {
		return nil, err
	}
	sender, ok := s.application.(application.SystemMessageSender)
	if !ok {
		return nil, status.Error(codes.Unimplemented, "system message sender is unavailable")
	}
	if err := sender.SendSystemGroupMessage(request.GetGroupId(), request.GetContent()); err != nil {
		return nil, rpcError(err)
	}
	return &messagev1.SendMessageResponse{}, nil
}

func requireCoreCaller(ctx context.Context, requestContext *commonv1.RequestContext) error {
	if _, err := principalFrom(ctx, requestContext); err != nil {
		return err
	}
	if caller, ok := grpcauth.CallerService(ctx); !ok || caller != "dipole-core" {
		return status.Error(codes.PermissionDenied, "only Core service may send system messages")
	}
	return nil
}

func (s *Server) GetMessageCommandReceipt(ctx context.Context, request *messagev1.GetMessageCommandReceiptRequest) (*messagev1.GetMessageCommandReceiptResponse, error) {
	ctx = grpccommon.Correlation(ctx, request.GetContext())
	principal, err := principalFrom(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	clientMessageID := strings.TrimSpace(request.GetClientMessageId())
	if clientMessageID == "" {
		return nil, rpcError(application.ErrMessageClientMessageIDInvalid)
	}
	query, ok := s.application.(application.MessageCommandReceiptQuery)
	if !ok {
		return nil, status.Error(codes.Internal, "Message Command receipt query is unavailable")
	}
	receipt, err := query.GetMessageCommandReceipt(principal, clientMessageID)
	if err != nil {
		return nil, rpcError(err)
	}
	if receipt == nil {
		return nil, status.Error(codes.Internal, "Message Command receipt is unavailable")
	}
	switch receipt.Status {
	case application.MessageCommandReceiptStatusAbsent:
		if receipt.Message != nil {
			return nil, status.Error(codes.Internal, "Message Command receipt has conflicting state")
		}
		return &messagev1.GetMessageCommandReceiptResponse{Status: messagev1.MessageCommandReceiptStatus_MESSAGE_COMMAND_RECEIPT_STATUS_ABSENT}, nil
	case application.MessageCommandReceiptStatusCommitted:
		if receipt.Message == nil || strings.TrimSpace(receipt.Message.SenderUUID) != principal || strings.TrimSpace(receipt.Message.ClientMessageID) != clientMessageID {
			return nil, status.Error(codes.Internal, "Message Command receipt has conflicting binding")
		}
		return &messagev1.GetMessageCommandReceiptResponse{
			Status:  messagev1.MessageCommandReceiptStatus_MESSAGE_COMMAND_RECEIPT_STATUS_COMMITTED,
			Message: grpcmapping.MessageToProto(receipt.Message),
		}, nil
	default:
		return nil, status.Error(codes.Internal, "Message Command receipt has unknown state")
	}
}

func (s *Server) ListDirectHistory(ctx context.Context, request *messagev1.ListDirectHistoryRequest) (*messagev1.ListMessagesResponse, error) {
	principal, err := principalFrom(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	pageSize, err := pageSize(request.GetPageSize())
	if err != nil {
		return nil, err
	}
	var messages []*model.Message
	var appErr error
	if request.BeforeSequence != nil && request.AfterSequence != nil {
		return nil, status.Error(codes.InvalidArgument, "before_sequence and after_sequence cannot be used together")
	}
	if request.AfterSequence != nil {
		if request.GetBeforeId() != 0 {
			return nil, status.Error(codes.InvalidArgument, "before_id and after_sequence cannot be used together")
		}
		messages, appErr = s.application.ListDirectMessagesAfterSeq(principal, request.GetTargetUserId(), request.GetAfterSequence(), pageSize)
	} else if request.BeforeSequence != nil {
		if request.GetBeforeId() != 0 {
			return nil, status.Error(codes.InvalidArgument, "before_id and before_sequence cannot be used together")
		}
		messages, appErr = s.application.ListDirectMessagesBeforeSeq(principal, request.GetTargetUserId(), request.GetBeforeSequence(), pageSize)
	} else {
		beforeID, cursorErr := uintCursor(request.GetBeforeId())
		if cursorErr != nil {
			return nil, cursorErr
		}
		messages, appErr = s.application.ListDirectMessages(principal, request.GetTargetUserId(), beforeID, pageSize)
	}
	if appErr != nil {
		return nil, rpcError(appErr)
	}
	return listResponse(messages), nil
}

func (s *Server) ListGroupHistory(ctx context.Context, request *messagev1.ListGroupHistoryRequest) (*messagev1.ListMessagesResponse, error) {
	principal, err := principalFrom(ctx, request.GetContext())
	if err != nil {
		return nil, err
	}
	limit, err := pageSize(request.GetPageSize())
	if err != nil {
		return nil, err
	}

	var messages []*model.Message
	switch cursor := request.GetCursor().(type) {
	case *messagev1.ListGroupHistoryRequest_BeforeSequence:
		messages, err = s.application.ListGroupMessagesBeforeSeq(principal, request.GetGroupId(), cursor.BeforeSequence, limit)
	case *messagev1.ListGroupHistoryRequest_AfterSequence:
		messages, err = s.application.ListGroupMessagesAfterSeq(principal, request.GetGroupId(), cursor.AfterSequence, limit)
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

func (s *Server) ListOfflineMessages(ctx context.Context, request *messagev1.ListOfflineMessagesRequest) (*messagev1.ListMessagesResponse, error) {
	principal, err := principalFrom(ctx, request.GetContext())
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

func principalFrom(ctx context.Context, invocation *commonv1.RequestContext) (string, error) {
	if _, err := grpccommon.Caller(ctx, invocation); err != nil {
		return "", err
	}
	return grpccommon.Principal(invocation)
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
	case errors.Is(err, application.ErrMessageClientMessageIDInvalid):
		return statusWithReason(codes.InvalidArgument, err, messagev1.ErrorReason_ERROR_REASON_CLIENT_MESSAGE_ID_INVALID)
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
		Message:          grpcmapping.MessageToProto(message),
		RecipientUserIds: recipients,
	}
}

func listResponse(messages []*model.Message) *messagev1.ListMessagesResponse {
	response := &messagev1.ListMessagesResponse{Messages: make([]*messagev1.Message, 0, len(messages))}
	for _, message := range messages {
		if message == nil {
			continue
		}
		response.Messages = append(response.Messages, grpcmapping.MessageToProto(message))
		if response.FirstId == 0 {
			response.FirstId = uint64(message.ID)
		}
		response.LastId = uint64(message.ID)
	}
	return response
}
