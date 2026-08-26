package syncgrpc

import (
	"context"
	"errors"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	grpccommon "github.com/JekYUlll/Dipole/internal/transport/grpc/common"
	syncv1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/sync/v1"
	grpcmapping "github.com/JekYUlll/Dipole/internal/transport/grpc/mapping"
)

const queryTimeout = 2 * time.Second

type Client struct {
	rpc syncv1.SyncQueryServiceClient
}

var _ application.SyncApplication = (*Client)(nil)

func NewClient(rpc syncv1.SyncQueryServiceClient) (*Client, error) {
	if rpc == nil {
		return nil, errors.New("sync query rpc client is required")
	}
	return &Client{rpc: rpc}, nil
}

func (c *Client) List(userUUID string, afterSeq uint64, limit int) (*application.SyncPage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	response, err := c.rpc.ListSyncMessages(ctx, &syncv1.ListSyncMessagesRequest{
		Context:  grpccommon.RequestContext(userUUID, "dipole-gateway"),
		AfterSeq: afterSeq,
		PageSize: requestPageSize(limit),
	})
	if err != nil {
		return nil, err
	}
	page := &application.SyncPage{
		Items:   make([]*model.SyncMessage, 0, len(response.GetItems())),
		NextSeq: response.GetNextSeq(),
		HasMore: response.GetHasMore(),
	}
	for _, item := range response.GetItems() {
		if item == nil {
			continue
		}
		page.Items = append(page.Items, &model.SyncMessage{
			SyncSeq:         item.GetSyncSeq(),
			ConversationKey: item.GetConversationKey(),
			Message:         grpcmapping.MessageFromProto(item.GetMessage()),
		})
	}
	return page, nil
}

func requestPageSize(limit int) int32 {
	switch {
	case limit <= 0:
		return 0
	case limit > 200:
		return 200
	default:
		return int32(limit)
	}
}
