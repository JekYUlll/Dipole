package syncgrpc

import (
	"context"
	"errors"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	grpccommon "github.com/JekYUlll/Dipole/internal/transport/grpc/common"
	commonv1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/common/v1"
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

func (c *Client) GetCheckpoint(userUUID, deviceID string) (*model.DeviceSyncCheckpoint, error) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	response, err := c.rpc.GetDeviceCheckpoint(ctx, &syncv1.GetDeviceCheckpointRequest{Context: syncRequestContext(userUUID, deviceID)})
	if err != nil {
		return nil, err
	}
	return &model.DeviceSyncCheckpoint{UserUUID: userUUID, DeviceID: response.GetDeviceId(), SyncSeq: response.GetSyncSeq()}, nil
}

func (c *Client) AdvanceCheckpoint(userUUID, deviceID string, syncSeq uint64) (*model.DeviceSyncCheckpoint, error) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	response, err := c.rpc.AdvanceDeviceCheckpoint(ctx, &syncv1.AdvanceDeviceCheckpointRequest{
		Context: syncRequestContext(userUUID, deviceID), SyncSeq: syncSeq,
	})
	if err != nil {
		return nil, err
	}
	return &model.DeviceSyncCheckpoint{UserUUID: userUUID, DeviceID: response.GetDeviceId(), SyncSeq: response.GetSyncSeq()}, nil
}

func syncRequestContext(userUUID, deviceID string) *commonv1.RequestContext {
	requestContext := grpccommon.RequestContext(userUUID, "dipole-gateway")
	requestContext.DeviceId = deviceID
	return requestContext
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
