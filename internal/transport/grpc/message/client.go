package messagegrpc

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	grpccommon "github.com/JekYUlll/Dipole/internal/transport/grpc/common"
	commonv1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/common/v1"
	messagev1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/message/v1"
	grpcmapping "github.com/JekYUlll/Dipole/internal/transport/grpc/mapping"
	"google.golang.org/grpc/status"
)

const (
	commandTimeout = 3 * time.Second
	queryTimeout   = 2 * time.Second
)

type Client struct {
	rpc           messagev1.MessageServiceClient
	callerService string
}

var _ application.MessageApplication = (*Client)(nil)

func NewClient(rpc messagev1.MessageServiceClient) (*Client, error) {
	return NewClientForService(rpc, "dipole-gateway")
}

func NewClientForService(rpc messagev1.MessageServiceClient, callerService string) (*Client, error) {
	if rpc == nil {
		return nil, errors.New("message rpc client is required")
	}
	callerService = strings.TrimSpace(callerService)
	if callerService == "" {
		return nil, errors.New("message rpc caller service is required")
	}
	return &Client{rpc: rpc, callerService: callerService}, nil
}

func (c *Client) SendDirectMessage(senderUUID, targetUUID, content, clientMessageID string) (*model.Message, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	response, err := c.rpc.SendDirectText(ctx, &messagev1.SendDirectTextRequest{
		Context:         c.invocation(senderUUID),
		TargetUserId:    targetUUID,
		Content:         content,
		ClientMessageId: clientMessageID,
	})
	if err != nil {
		return nil, domainError(err)
	}
	return grpcmapping.MessageFromProto(response.GetMessage()), nil
}

func (c *Client) SendGroupMessage(senderUUID, groupUUID, content, clientMessageID string) (*model.Message, []string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	response, err := c.rpc.SendGroupText(ctx, &messagev1.SendGroupTextRequest{
		Context:         c.invocation(senderUUID),
		GroupId:         groupUUID,
		Content:         content,
		ClientMessageId: clientMessageID,
	})
	if err != nil {
		return nil, nil, domainError(err)
	}
	return grpcmapping.MessageFromProto(response.GetMessage()), response.GetRecipientUserIds(), nil
}

func (c *Client) SendDirectFileMessage(senderUUID, targetUUID, fileUUID, clientMessageID string) (*model.Message, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	response, err := c.rpc.SendDirectFile(ctx, &messagev1.SendDirectFileRequest{
		Context:         c.invocation(senderUUID),
		TargetUserId:    targetUUID,
		FileId:          fileUUID,
		ClientMessageId: clientMessageID,
	})
	if err != nil {
		return nil, domainError(err)
	}
	return grpcmapping.MessageFromProto(response.GetMessage()), nil
}

func (c *Client) SendGroupFileMessage(senderUUID, groupUUID, fileUUID, clientMessageID string) (*model.Message, []string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	response, err := c.rpc.SendGroupFile(ctx, &messagev1.SendGroupFileRequest{
		Context:         c.invocation(senderUUID),
		GroupId:         groupUUID,
		FileId:          fileUUID,
		ClientMessageId: clientMessageID,
	})
	if err != nil {
		return nil, nil, domainError(err)
	}
	return grpcmapping.MessageFromProto(response.GetMessage()), response.GetRecipientUserIds(), nil
}

func (c *Client) ListDirectMessages(currentUserUUID, targetUUID string, beforeID uint, limit int) ([]*model.Message, error) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	response, err := c.rpc.ListDirectHistory(ctx, &messagev1.ListDirectHistoryRequest{
		Context:      c.invocation(currentUserUUID),
		TargetUserId: targetUUID,
		BeforeId:     uint64(beforeID),
		PageSize:     requestPageSize(limit),
	})
	if err != nil {
		return nil, domainError(err)
	}
	return messagesFromProto(response.GetMessages()), nil
}

func (c *Client) ListDirectMessagesBeforeSeq(currentUserUUID, targetUUID string, beforeSeq uint64, limit int) ([]*model.Message, error) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	response, err := c.rpc.ListDirectHistory(ctx, &messagev1.ListDirectHistoryRequest{
		Context: c.invocation(currentUserUUID), TargetUserId: targetUUID,
		BeforeSequence: &beforeSeq, PageSize: requestPageSize(limit),
	})
	if err != nil {
		return nil, domainError(err)
	}
	return messagesFromProto(response.GetMessages()), nil
}

func (c *Client) ListDirectMessagesAfterSeq(currentUserUUID, targetUUID string, afterSeq uint64, limit int) ([]*model.Message, error) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	response, err := c.rpc.ListDirectHistory(ctx, &messagev1.ListDirectHistoryRequest{
		Context: c.invocation(currentUserUUID), TargetUserId: targetUUID,
		AfterSequence: &afterSeq, PageSize: requestPageSize(limit),
	})
	if err != nil {
		return nil, domainError(err)
	}
	return messagesFromProto(response.GetMessages()), nil
}

func (c *Client) ListGroupMessages(currentUserUUID, groupUUID string, beforeID uint, limit int) ([]*model.Message, error) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	request := &messagev1.ListGroupHistoryRequest{
		Context:  c.invocation(currentUserUUID),
		GroupId:  groupUUID,
		PageSize: requestPageSize(limit),
	}
	if beforeID > 0 {
		request.Cursor = &messagev1.ListGroupHistoryRequest_BeforeId{BeforeId: uint64(beforeID)}
	}
	response, err := c.rpc.ListGroupHistory(ctx, request)
	if err != nil {
		return nil, domainError(err)
	}
	return messagesFromProto(response.GetMessages()), nil
}

func (c *Client) ListGroupMessagesBeforeSeq(currentUserUUID, groupUUID string, beforeSeq uint64, limit int) ([]*model.Message, error) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	response, err := c.rpc.ListGroupHistory(ctx, &messagev1.ListGroupHistoryRequest{
		Context: c.invocation(currentUserUUID), GroupId: groupUUID,
		Cursor: &messagev1.ListGroupHistoryRequest_BeforeSequence{BeforeSequence: beforeSeq}, PageSize: requestPageSize(limit),
	})
	if err != nil {
		return nil, domainError(err)
	}
	return messagesFromProto(response.GetMessages()), nil
}

func (c *Client) ListGroupMessagesAfter(currentUserUUID, groupUUID string, afterID uint, limit int) ([]*model.Message, error) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	response, err := c.rpc.ListGroupHistory(ctx, &messagev1.ListGroupHistoryRequest{
		Context:  c.invocation(currentUserUUID),
		GroupId:  groupUUID,
		Cursor:   &messagev1.ListGroupHistoryRequest_AfterId{AfterId: uint64(afterID)},
		PageSize: requestPageSize(limit),
	})
	if err != nil {
		return nil, domainError(err)
	}
	return messagesFromProto(response.GetMessages()), nil
}

func (c *Client) ListGroupMessagesAfterSeq(currentUserUUID, groupUUID string, afterSeq uint64, limit int) ([]*model.Message, error) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	response, err := c.rpc.ListGroupHistory(ctx, &messagev1.ListGroupHistoryRequest{
		Context: c.invocation(currentUserUUID), GroupId: groupUUID,
		Cursor: &messagev1.ListGroupHistoryRequest_AfterSequence{AfterSequence: afterSeq}, PageSize: requestPageSize(limit),
	})
	if err != nil {
		return nil, domainError(err)
	}
	return messagesFromProto(response.GetMessages()), nil
}

func (c *Client) ListOfflineMessages(currentUserUUID string, afterID uint, limit int) ([]*model.Message, error) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	response, err := c.rpc.ListOfflineMessages(ctx, &messagev1.ListOfflineMessagesRequest{
		Context:  c.invocation(currentUserUUID),
		AfterId:  uint64(afterID),
		PageSize: requestPageSize(limit),
	})
	if err != nil {
		return nil, domainError(err)
	}
	return messagesFromProto(response.GetMessages()), nil
}

func (c *Client) invocation(principal string) *commonv1.RequestContext {
	return grpccommon.RequestContext(principal, c.callerService)
}

func requestPageSize(limit int) int32 {
	switch {
	case limit <= 0:
		return 0
	case limit > 50:
		return 50
	default:
		return int32(limit)
	}
}

func domainError(err error) error {
	grpcStatus, ok := status.FromError(err)
	if !ok {
		return err
	}
	for _, detail := range grpcStatus.Details() {
		errorDetail, detailOK := detail.(*messagev1.ErrorDetail)
		if !detailOK {
			continue
		}
		if mapped := errorForReason(errorDetail.GetReason()); mapped != nil {
			return mapped
		}
	}
	return err
}

func errorForReason(reason messagev1.ErrorReason) error {
	switch reason {
	case messagev1.ErrorReason_ERROR_REASON_TARGET_REQUIRED:
		return application.ErrMessageTargetRequired
	case messagev1.ErrorReason_ERROR_REASON_CONTENT_REQUIRED:
		return application.ErrMessageContentRequired
	case messagev1.ErrorReason_ERROR_REASON_CONTENT_TOO_LONG:
		return application.ErrMessageContentTooLong
	case messagev1.ErrorReason_ERROR_REASON_TARGET_UNAVAILABLE:
		return application.ErrMessageTargetUnavailable
	case messagev1.ErrorReason_ERROR_REASON_TARGET_NOT_FOUND:
		return application.ErrMessageTargetNotFound
	case messagev1.ErrorReason_ERROR_REASON_FRIEND_REQUIRED:
		return application.ErrMessageFriendRequired
	case messagev1.ErrorReason_ERROR_REASON_GROUP_FORBIDDEN:
		return application.ErrMessageGroupForbidden
	case messagev1.ErrorReason_ERROR_REASON_FILE_REQUIRED:
		return application.ErrMessageFileRequired
	case messagev1.ErrorReason_ERROR_REASON_FILE_UNAVAILABLE:
		return application.ErrMessageFileUnavailable
	case messagev1.ErrorReason_ERROR_REASON_IDEMPOTENCY_CONFLICT:
		return application.ErrMessageIdempotencyConflict
	default:
		return nil
	}
}

func messagesFromProto(messages []*messagev1.Message) []*model.Message {
	result := make([]*model.Message, 0, len(messages))
	for _, message := range messages {
		if message != nil {
			result = append(result, grpcmapping.MessageFromProto(message))
		}
	}
	return result
}
