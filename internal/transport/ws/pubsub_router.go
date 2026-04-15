package ws

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/JekYUlll/Dipole/internal/logger"
	presencePkg "github.com/JekYUlll/Dipole/internal/platform/presence"
)

const pubsubPublishTimeout = time.Second

// nodeMessageKind 枚举跨节点消息类型
const (
	nodeMessageKindEvent         = "event"
	nodeMessageKindDisconnect    = "disconnect"
	nodeMessageKindDisconnectAll = "disconnect_all"
)

// NodeMessage 是发布到 ws:node:{nodeID} channel 的消息体。
// Payload 存放已序列化的 OutboundEvent，接收端直接调用 hub.sendToUser，避免二次序列化。
type NodeMessage struct {
	Kind          string   `json:"kind"`                     // "event" | "disconnect" | "disconnect_all"
	UserUUID      string   `json:"user_uuid"`
	EventType     string   `json:"event_type,omitempty"`
	Payload       []byte   `json:"payload,omitempty"`        // 已序列化的 OutboundEvent（kind=event 时使用）
	ConnectionIDs []string `json:"connection_ids,omitempty"` // kind=disconnect 时指定要踢的连接
	Reason        string   `json:"reason,omitempty"`
}

// presenceReader 是 PubSubRouter 对 RedisPresence 的最小依赖接口。
// *presence.RedisPresence 已满足此接口，无需修改。
type presenceReader interface {
	ListUserConnections(userUUID string) ([]presencePkg.ConnectionState, error)
	NodeID() string
}

// PubSubRouter 通过 Redis Pub/Sub 实现跨节点 WebSocket 事件投递。
//
// 工作原理：
//   - 每个节点订阅自己的 channel ws:node:{nodeID}
//   - SendEventToUser 先查 presence 确定目标用户的连接分布，
//     本节点连接直接投递，远端节点连接通过 PUBLISH 转发
//   - 接收到其他节点发来的消息后，dispatch 到本地 hub
//
// 仅在 Kafka + Presence 同时启用时激活；单节点模式下 NewPubSubRouter 返回 nil，
// 调用方降级为直接使用 *Hub。
type PubSubRouter struct {
	hub      *Hub
	presence presenceReader
	nodeID   string
	rdb      *redis.Client
	log      *zap.Logger
	stopCh   chan struct{}
}

// NewPubSubRouter 创建跨节点路由器。
// 任意参数为 nil，或 presence.NodeID() 为空时返回 nil（调用方降级为本地投递）。
func NewPubSubRouter(hub *Hub, p presenceReader, rdb *redis.Client) *PubSubRouter {
	if hub == nil || p == nil || rdb == nil {
		return nil
	}
	nodeID := p.NodeID()
	if nodeID == "" {
		return nil
	}
	return &PubSubRouter{
		hub:      hub,
		presence: p,
		nodeID:   nodeID,
		rdb:      rdb,
		log:      logger.Named("ws.pubsub"),
		stopCh:   make(chan struct{}),
	}
}

// Start 订阅本节点的 Redis channel 并在后台 goroutine 中处理入站消息。
// 应在 Initialize 完成后、服务开始接受请求前调用一次。
func (r *PubSubRouter) Start() {
	sub := r.rdb.Subscribe(context.Background(), nodeChannel(r.nodeID))
	go r.loop(sub)
}

// Stop 关闭订阅 goroutine。由 Runtime.Close() 调用。
func (r *PubSubRouter) Stop() {
	close(r.stopCh)
}

// SendEventToUser 实现 kafkaWSEventSender 接口。
// 查询 presence 确定目标用户的连接分布：
//   - 本节点连接：直接调用 hub.sendToUser
//   - 远端节点连接：PUBLISH 到对应节点的 channel
//
// 若 presence 查询失败或返回空（用户离线），降级为本地投递。
// 返回本节点实际投递的连接数（远端投递为 fire-and-forget，不计入返回值）。
func (r *PubSubRouter) SendEventToUser(userUUID, eventType string, data any) int {
	payload, err := json.Marshal(OutboundEvent{Type: eventType, Data: data})
	if err != nil {
		r.log.Warn("pubsub marshal outbound event failed",
			zap.String("user_uuid", userUUID),
			zap.String("event_type", eventType),
			zap.Error(err),
		)
		return 0
	}

	connections, err := r.presence.ListUserConnections(userUUID)
	if err != nil || len(connections) == 0 {
		// presence 不可用或用户离线，降级为本地投递
		return r.hub.sendToUser(userUUID, payload)
	}

	// 按节点分组
	hasLocal := false
	remoteNodes := make(map[string]struct{})
	for _, c := range connections {
		if c.NodeID == r.nodeID {
			hasLocal = true
		} else if c.NodeID != "" {
			remoteNodes[c.NodeID] = struct{}{}
		}
	}

	delivered := 0
	if hasLocal {
		delivered = r.hub.sendToUser(userUUID, payload)
	}
	for targetNodeID := range remoteNodes {
		r.publish(targetNodeID, NodeMessage{
			Kind:      nodeMessageKindEvent,
			UserUUID:  userUUID,
			EventType: eventType,
			Payload:   payload,
		})
	}

	return delivered
}

// DisconnectConnections 实现 kafkaWSEventSender 接口。
// 将 connectionIDs 按节点分组，本节点直接踢，远端节点通过 Pub/Sub 转发。
func (r *PubSubRouter) DisconnectConnections(userUUID string, connectionIDs []string, reason string) int {
	if len(connectionIDs) == 0 {
		return 0
	}

	connections, err := r.presence.ListUserConnections(userUUID)
	if err != nil || len(connections) == 0 {
		return r.hub.DisconnectConnections(userUUID, connectionIDs, reason)
	}

	// 建立 connectionID → nodeID 映射
	connNode := make(map[string]string, len(connections))
	for _, c := range connections {
		connNode[c.ConnectionID] = c.NodeID
	}

	localIDs := make([]string, 0)
	remoteIDs := make(map[string][]string) // nodeID → []connectionID
	for _, id := range connectionIDs {
		nodeID, ok := connNode[id]
		if !ok || nodeID == r.nodeID {
			localIDs = append(localIDs, id)
		} else {
			remoteIDs[nodeID] = append(remoteIDs[nodeID], id)
		}
	}

	disconnected := 0
	if len(localIDs) > 0 {
		disconnected = r.hub.DisconnectConnections(userUUID, localIDs, reason)
	}
	for targetNodeID, ids := range remoteIDs {
		r.publish(targetNodeID, NodeMessage{
			Kind:          nodeMessageKindDisconnect,
			UserUUID:      userUUID,
			ConnectionIDs: ids,
			Reason:        reason,
		})
	}

	return disconnected
}

// DisconnectAllConnections 实现 kafkaWSEventSender 接口。
// 本节点直接踢，同时向所有持有该用户连接的远端节点广播 disconnect_all。
func (r *PubSubRouter) DisconnectAllConnections(userUUID string, reason string) int {
	connections, err := r.presence.ListUserConnections(userUUID)
	if err != nil || len(connections) == 0 {
		return r.hub.DisconnectAllConnections(userUUID, reason)
	}

	remoteNodes := make(map[string]struct{})
	for _, c := range connections {
		if c.NodeID != r.nodeID && c.NodeID != "" {
			remoteNodes[c.NodeID] = struct{}{}
		}
	}

	disconnected := r.hub.DisconnectAllConnections(userUUID, reason)
	for targetNodeID := range remoteNodes {
		r.publish(targetNodeID, NodeMessage{
			Kind:     nodeMessageKindDisconnectAll,
			UserUUID: userUUID,
			Reason:   reason,
		})
	}

	return disconnected
}

// loop 是订阅 goroutine 的主循环，监听本节点 channel 并分发消息。
func (r *PubSubRouter) loop(sub *redis.PubSub) {
	defer sub.Close()
	ch := sub.Channel()
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				// Redis 连接断开，channel 被关闭，goroutine 自然退出
				return
			}
			r.dispatch(msg.Payload)
		case <-r.stopCh:
			return
		}
	}
}

// dispatch 解析入站 NodeMessage 并调用本地 hub 方法完成投递。
func (r *PubSubRouter) dispatch(raw string) {
	var msg NodeMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		r.log.Warn("pubsub dispatch unmarshal failed", zap.Error(err))
		return
	}

	switch msg.Kind {
	case nodeMessageKindEvent:
		// Payload 已是序列化好的 OutboundEvent，直接投递，无需再次序列化
		r.hub.sendToUser(msg.UserUUID, msg.Payload)
	case nodeMessageKindDisconnect:
		r.hub.DisconnectConnections(msg.UserUUID, msg.ConnectionIDs, msg.Reason)
	case nodeMessageKindDisconnectAll:
		r.hub.DisconnectAllConnections(msg.UserUUID, msg.Reason)
	default:
		r.log.Warn("pubsub unknown message kind", zap.String("kind", msg.Kind))
	}
}

// publish 将 NodeMessage 序列化后发布到目标节点的 Redis channel。
// 发布失败只记录 warn 日志，不阻塞调用方（Kafka 消费协程）。
func (r *PubSubRouter) publish(targetNodeID string, msg NodeMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		r.log.Warn("pubsub publish marshal failed",
			zap.String("target_node", targetNodeID),
			zap.Error(err),
		)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), pubsubPublishTimeout)
	defer cancel()

	if err := r.rdb.Publish(ctx, nodeChannel(targetNodeID), data).Err(); err != nil {
		r.log.Warn("pubsub publish failed",
			zap.String("target_node", targetNodeID),
			zap.String("channel", nodeChannel(targetNodeID)),
			zap.Error(err),
		)
	}
}

// nodeChannel 返回节点对应的 Redis Pub/Sub channel 名称。
func nodeChannel(nodeID string) string {
	return "ws:node:" + nodeID
}
