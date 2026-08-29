package syncgrpc

import (
	"context"
	"errors"
	"strings"
	"time"

	commonv1 "github.com/JekYUlll/Dipole/api/gen/go/common/v1"
	syncv1 "github.com/JekYUlll/Dipole/api/gen/go/sync/v1"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	grpccommon "github.com/JekYUlll/Dipole/internal/transport/grpc/common"
	grpcmapping "github.com/JekYUlll/Dipole/internal/transport/grpc/mapping"
)

const queryTimeout = 2 * time.Second

type Client struct {
	rpc     syncv1.SyncQueryServiceClient
	service string
}

var _ application.SyncApplication = (*Client)(nil)

func NewClient(rpc syncv1.SyncQueryServiceClient) (*Client, error) {
	return NewClientForService(rpc, "dipole-gateway")
}

func NewClientForService(rpc syncv1.SyncQueryServiceClient, service string) (*Client, error) {
	if rpc == nil {
		return nil, errors.New("sync query rpc client is required")
	}
	service = strings.TrimSpace(service)
	if service == "" {
		return nil, errors.New("sync query caller service is required")
	}
	return &Client{rpc: rpc, service: service}, nil
}

func (c *Client) List(userUUID string, afterSeq uint64, limit int) (*application.SyncPage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	response, err := c.rpc.ListSyncMessages(ctx, &syncv1.ListSyncMessagesRequest{
		Context:  grpccommon.RequestContext(userUUID, c.service),
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
		message := grpcmapping.MessageFromProto(item.GetMessage())
		messageID, messageSequence := item.GetMessageUuid(), item.GetMessageSeq()
		if message != nil {
			if messageID == "" {
				messageID = message.UUID
			}
			if messageSequence == 0 {
				messageSequence = message.Seq
			}
		}
		page.Items = append(page.Items, &model.SyncMessage{
			SyncSeq:         item.GetSyncSeq(),
			ConversationKey: item.GetConversationKey(),
			MessageUUID:     messageID,
			MessageSeq:      messageSequence,
			Message:         message,
		})
	}
	return page, nil
}

func (c *Client) GetCheckpoint(userUUID, deviceID string) (*model.DeviceSyncCheckpoint, error) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	response, err := c.rpc.GetDeviceCheckpoint(ctx, &syncv1.GetDeviceCheckpointRequest{Context: syncRequestContext(userUUID, deviceID, c.service)})
	if err != nil {
		return nil, err
	}
	return &model.DeviceSyncCheckpoint{UserUUID: userUUID, DeviceID: response.GetDeviceId(), SyncSeq: response.GetSyncSeq()}, nil
}

func (c *Client) AdvanceCheckpoint(userUUID, deviceID string, syncSeq uint64) (*model.DeviceSyncCheckpoint, error) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	response, err := c.rpc.AdvanceDeviceCheckpoint(ctx, &syncv1.AdvanceDeviceCheckpointRequest{
		Context: syncRequestContext(userUUID, deviceID, c.service), SyncSeq: syncSeq,
	})
	if err != nil {
		return nil, err
	}
	return &model.DeviceSyncCheckpoint{UserUUID: userUUID, DeviceID: response.GetDeviceId(), SyncSeq: response.GetSyncSeq()}, nil
}

func (c *Client) ListGroupCheckpoints(userUUID, deviceID string, groupUUIDs []string) ([]*model.GroupSyncCheckpoint, error) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	response, err := c.rpc.ListGroupCheckpoints(ctx, &syncv1.ListGroupCheckpointsRequest{Context: syncRequestContext(userUUID, deviceID, c.service), GroupIds: groupUUIDs})
	if err != nil {
		return nil, err
	}
	result := make([]*model.GroupSyncCheckpoint, 0, len(response.GetCheckpoints()))
	for _, checkpoint := range response.GetCheckpoints() {
		if checkpoint != nil {
			result = append(result, groupCheckpointFromProto(checkpoint))
		}
	}
	return result, nil
}

func (c *Client) AdvanceGroupCheckpoint(userUUID, deviceID, groupUUID string, messageSeq uint64) (*model.GroupSyncCheckpoint, error) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	response, err := c.rpc.AdvanceGroupCheckpoint(ctx, &syncv1.AdvanceGroupCheckpointRequest{
		Context: syncRequestContext(userUUID, deviceID, c.service), GroupId: groupUUID, MessageSequence: messageSeq,
	})
	if err != nil {
		return nil, err
	}
	return groupCheckpointFromProto(response), nil
}

func groupCheckpointFromProto(checkpoint *syncv1.GroupCheckpoint) *model.GroupSyncCheckpoint {
	if checkpoint == nil {
		return &model.GroupSyncCheckpoint{}
	}
	return &model.GroupSyncCheckpoint{GroupUUID: checkpoint.GetGroupId(), LatestMessageSeq: checkpoint.GetLatestMessageSequence(), LatestMessageUUID: checkpoint.GetLatestMessageId(), PulledMessageSeq: checkpoint.GetPulledMessageSequence()}
}

func syncRequestContext(userUUID, deviceID, callerService string) *commonv1.RequestContext {
	requestContext := grpccommon.RequestContext(userUUID, callerService)
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
