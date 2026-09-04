package kafka

import (
	"context"
	"fmt"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/logger"
	"github.com/JekYUlll/Dipole/internal/platform/cache"
	platformHotGroup "github.com/JekYUlll/Dipole/internal/platform/hotgroup"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	realtimeDelivery "github.com/JekYUlll/Dipole/internal/realtime/delivery"
	wsTransport "github.com/JekYUlll/Dipole/internal/transport/ws"
	"go.uber.org/zap"
)

type RealtimeHub interface {
	EventSender
	ConnectionController
}

// RegisterHandlers installs all Kafka consumers owned by Gateway.
func RegisterHandlers(hub RealtimeHub, authority realtimeDelivery.Authority, fence realtimeDelivery.AuthorityFence, agentTaskWaitingObservers ...AgentTaskWaitingObserver) error {
	if platformKafka.Subscriber == nil {
		return nil
	}
	if hub == nil {
		return fmt.Errorf("gateway kafka event sender is required")
	}

	hotGroups := platformHotGroup.NewDetectorWithClient(config.HotGroupConfig(), cache.RDB)
	notifier := NewNotifier(hub, DefaultNotifyWindow)
	platformKafka.Subscriber.Register("group.created", NewGroupEventHandler(hub, wsTransport.TypeGroupCreated, func(p application.GroupEventPayload) wsTransport.GroupCreatedEventData {
		return wsTransport.GroupCreatedEventData{GroupUUID: p.GroupUUID, Name: p.Name, Notice: p.Notice, Avatar: p.Avatar, MemberUUIDs: p.MemberUUIDs, OperatorUUID: p.OperatorUUID, OccurredAt: p.OccurredAt}
	}))

	directHandler, groupHandler, err := NewMessageDeliveryHandlers(authority, hub, hotGroups, notifier, config.MessageConfig().TimelineNotifyMode)
	if err != nil {
		return err
	}
	platformKafka.Subscriber.Register("message.direct.created", FenceMessageDeliveryHandler(authority, fence, directHandler))
	platformKafka.Subscriber.Register("message.group.created", FenceMessageDeliveryHandler(authority, fence, groupHandler))
	platformKafka.Subscriber.Register("conversation.direct.read", NewDirectReadHandler(hub))
	platformKafka.Subscriber.Register("group.updated", NewGroupEventHandler(hub, wsTransport.TypeGroupUpdated, func(p application.GroupEventPayload) wsTransport.GroupUpdatedEventData {
		return wsTransport.GroupUpdatedEventData{GroupUUID: p.GroupUUID, Name: p.Name, Notice: p.Notice, Avatar: p.Avatar, OperatorUUID: p.OperatorUUID, UpdatedAt: p.OccurredAt}
	}))
	platformKafka.Subscriber.Register("group.members.added", NewGroupEventHandler(hub, wsTransport.TypeGroupMembersAdded, func(p application.GroupEventPayload) wsTransport.GroupMembersChangedEventData {
		return wsTransport.GroupMembersChangedEventData{GroupUUID: p.GroupUUID, MemberUUIDs: p.MemberUUIDs, OperatorUUID: p.OperatorUUID, OccurredAt: p.OccurredAt}
	}))
	platformKafka.Subscriber.Register("group.members.removed", NewGroupEventHandler(hub, wsTransport.TypeGroupMembersRemoved, func(p application.GroupEventPayload) wsTransport.GroupMembersChangedEventData {
		return wsTransport.GroupMembersChangedEventData{GroupUUID: p.GroupUUID, MemberUUIDs: p.MemberUUIDs, OperatorUUID: p.OperatorUUID, OccurredAt: p.OccurredAt}
	}))
	platformKafka.Subscriber.Register("group.dismissed", NewGroupEventHandler(hub, wsTransport.TypeGroupDismissed, func(p application.GroupEventPayload) wsTransport.GroupDismissedEventData {
		return wsTransport.GroupDismissedEventData{GroupUUID: p.GroupUUID, GroupName: p.GroupName, OperatorUUID: p.OperatorUUID, OccurredAt: p.OccurredAt}
	}))
	platformKafka.Subscriber.Register("session.force_logout", NewSessionKickHandler(hub))
	platformKafka.Subscriber.Register("contact.friend.deleted", NewContactFriendDeletedHandler(hub))
	platformKafka.Subscriber.Register(application.AgentTaskWaitingEventTypeV1, NewAgentTaskWaitingHandler(hub, agentTaskWaitingObservers...))
	return nil
}

// NewMessageDeliveryHandlers selects the Go/shadow or C++ checkpoint path.
func NewMessageDeliveryHandlers(
	authority realtimeDelivery.Authority,
	hub EventSender,
	hotGroups GroupHeatReader,
	notifier *Notifier,
	timelineNotifyMode string,
) (platformKafka.Handler, platformKafka.Handler, error) {
	switch authority {
	case realtimeDelivery.AuthorityGo, realtimeDelivery.AuthorityShadow:
		return NewDirectMessageHandler(hub, timelineNotifyMode), NewGroupMessageHandler(hub, hotGroups, notifier, timelineNotifyMode), nil
	case realtimeDelivery.AuthorityCPP:
		return checkpointMessageDeliveryHandler("direct"), checkpointMessageDeliveryHandler("group"), nil
	default:
		return nil, nil, fmt.Errorf("unsupported Gateway realtime delivery authority %q", authority)
	}
}

func checkpointMessageDeliveryHandler(label string) platformKafka.Handler {
	return func(_ context.Context, event platformKafka.Event) error {
		if _, err := decodeMessageEventPayload(event); err != nil {
			logger.Warn("decode "+label+" message for delivery checkpoint failed", zap.Error(err))
			return err
		}
		return nil
	}
}
