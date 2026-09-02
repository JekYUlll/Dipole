package searchgrpc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	searchv1 "github.com/JekYUlll/Dipole/api/gen/go/search/v1"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	grpccommon "github.com/JekYUlll/Dipole/internal/transport/grpc/common"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const queryTimeout = 2 * time.Second

type Client struct {
	rpc           searchv1.SearchServiceClient
	callerService string
}

var _ application.SearchApplication = (*Client)(nil)

func NewClient(rpc searchv1.SearchServiceClient) (*Client, error) {
	return NewClientForService(rpc, "dipole-gateway")
}

func NewClientForService(rpc searchv1.SearchServiceClient, callerService string) (*Client, error) {
	if rpc == nil {
		return nil, errors.New("Search rpc client is required")
	}
	callerService = strings.TrimSpace(callerService)
	if callerService == "" {
		return nil, errors.New("Search rpc caller service is required")
	}
	return &Client{rpc: rpc, callerService: callerService}, nil
}

func (c *Client) Search(principal, text string, limit int) ([]*model.MessageSearchDocument, error) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	response, err := c.rpc.SearchMessages(ctx, &searchv1.SearchMessagesRequest{
		Context:  grpccommon.RequestContext(principal, c.callerService),
		Query:    text,
		PageSize: searchPageSize(limit),
	})
	if err != nil {
		if status.Code(err) == codes.InvalidArgument && status.Convert(err).Message() == application.ErrSearchTextRequired.Error() {
			return nil, application.ErrSearchTextRequired
		}
		return nil, err
	}
	documents := make([]*model.MessageSearchDocument, 0, len(response.GetMessages()))
	for _, message := range response.GetMessages() {
		if message == nil {
			continue
		}
		if message.GetSentAt() == nil {
			return nil, fmt.Errorf("Search response message %s is missing sent_at", message.GetServerMessageId())
		}
		documents = append(documents, &model.MessageSearchDocument{
			MessageUUID: message.GetServerMessageId(), ConversationKey: message.GetConversationKey(),
			MessageSeq: message.GetSequence(), Revision: message.GetRevision(), SenderUUID: message.GetSenderId(),
			MessageType: int8(message.GetMessageType()), Content: message.GetContent(), SentAt: message.GetSentAt().AsTime(),
		})
	}
	return documents, nil
}

func searchPageSize(limit int) int32 {
	switch {
	case limit <= 0:
		return 0
	case limit > 100:
		return 100
	default:
		return int32(limit)
	}
}
