package deliverygrpc

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	deliveryv1 "github.com/JekYUlll/Dipole/api/gen/go/delivery/v1"
	deliverycontract "github.com/JekYUlll/Dipole/internal/realtime/delivery"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ObservationSink interface {
	Observe(*deliveryv1.NodeDeliveryBatch)
}

type ShadowServer struct {
	deliveryv1.UnimplementedNodeDeliveryServiceServer
	nodeID         string
	retryAfter     time.Duration
	sink           ObservationSink
	queue          chan *deliveryv1.NodeDeliveryBatch
	dedupeCapacity int
	dedupeMu       sync.Mutex
	seen           map[string]struct{}
	seenOrder      []string
	closed         atomic.Bool
	closeOnce      sync.Once
	workerDone     chan struct{}
	primary        *PrimaryDispatcher
}

func (s *ShadowServer) EnablePrimary(dispatcher *PrimaryDispatcher) error {
	if s == nil || dispatcher == nil {
		return errors.New("primary delivery dispatcher is required")
	}
	if dispatcher.nodeID != s.nodeID {
		return errors.New("primary delivery dispatcher node does not match receiver")
	}
	s.dedupeMu.Lock()
	defer s.dedupeMu.Unlock()
	if s.closed.Load() {
		return errors.New("node delivery receiver is closed")
	}
	if s.primary != nil {
		return errors.New("primary delivery dispatcher is already enabled")
	}
	s.primary = dispatcher
	return nil
}

func (s *ShadowServer) DeliverNodeBatch(ctx context.Context, batch *deliveryv1.NodeDeliveryBatch) (*deliveryv1.DeliveryAck, error) {
	if s == nil {
		return nil, status.Error(codes.Unavailable, "node delivery receiver is closed")
	}
	s.dedupeMu.Lock()
	defer s.dedupeMu.Unlock()
	if s.closed.Load() {
		return nil, status.Error(codes.Unavailable, "node delivery receiver is closed")
	}
	primary := s.primary
	if primary == nil {
		return nil, status.Error(codes.FailedPrecondition, "primary node delivery is disabled")
	}
	return primary.DeliverNodeBatch(ctx, batch)
}

func NewShadowServer(nodeID string, capacity int, retryAfter time.Duration, sink ObservationSink) (*ShadowServer, error) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" || capacity <= 0 || retryAfter <= 0 || retryAfter > time.Minute || sink == nil {
		return nil, errors.New("node observation server requires node, capacity, retry delay, and sink")
	}
	dedupeCapacity := capacity * 4
	if dedupeCapacity < 64 {
		dedupeCapacity = 64
	}
	server := &ShadowServer{
		nodeID: nodeID, retryAfter: retryAfter, sink: sink,
		queue: make(chan *deliveryv1.NodeDeliveryBatch, capacity), dedupeCapacity: dedupeCapacity,
		seen: make(map[string]struct{}, dedupeCapacity), seenOrder: make([]string, 0, dedupeCapacity),
		workerDone: make(chan struct{}),
	}
	go server.run()
	return server, nil
}

func (s *ShadowServer) ObserveNodeBatch(_ context.Context, batch *deliveryv1.NodeDeliveryBatch) (*deliveryv1.NodeDeliveryObservation, error) {
	if s == nil || s.closed.Load() {
		return nil, status.Error(codes.Unavailable, "node observation receiver is closed")
	}
	if batch == nil || strings.TrimSpace(batch.BatchId) == "" || strings.TrimSpace(batch.TargetNodeId) == "" {
		return nil, status.Error(codes.InvalidArgument, "node delivery batch identity is required")
	}
	if err := deliverycontract.ValidateNodeBatch(batch); err != nil || batch.TargetNodeId != s.nodeID {
		return s.rejected(batch), nil
	}

	items, connections := batchCounts(batch)
	s.dedupeMu.Lock()
	defer s.dedupeMu.Unlock()
	if s.closed.Load() {
		return nil, status.Error(codes.Unavailable, "node observation receiver is closed")
	}
	if _, exists := s.seen[batch.BatchId]; exists {
		return s.observed(batch, items, connections, true), nil
	}
	clone := proto.Clone(batch).(*deliveryv1.NodeDeliveryBatch)
	select {
	case s.queue <- clone:
		s.remember(batch.BatchId)
		return s.observed(batch, items, connections, false), nil
	default:
		return &deliveryv1.NodeDeliveryObservation{
			ContractVersion: deliverycontract.ContractVersion,
			BatchId:         batch.BatchId, TargetNodeId: s.nodeID,
			Status:     deliveryv1.NodeObservationStatus_NODE_OBSERVATION_STATUS_BACKPRESSURED,
			Pressure:   &deliveryv1.QueuePressure{Depth: uint32(len(s.queue)), Capacity: uint32(cap(s.queue)), RetryAfterMs: uint32(s.retryAfter.Milliseconds())},
			ErrorCode:  deliveryv1.DeliveryErrorCode_DELIVERY_ERROR_CODE_QUEUE_FULL,
			ObservedAt: timestamppb.Now(),
		}, nil
	}
}

func (s *ShadowServer) Close() {
	if s == nil {
		return
	}
	s.closeOnce.Do(func() {
		s.dedupeMu.Lock()
		s.closed.Store(true)
		close(s.queue)
		s.dedupeMu.Unlock()
		<-s.workerDone
	})
}

func (s *ShadowServer) run() {
	defer close(s.workerDone)
	for batch := range s.queue {
		s.sink.Observe(batch)
	}
}

func (s *ShadowServer) remember(batchID string) {
	s.seen[batchID] = struct{}{}
	s.seenOrder = append(s.seenOrder, batchID)
	if len(s.seenOrder) > s.dedupeCapacity {
		delete(s.seen, s.seenOrder[0])
		s.seenOrder = s.seenOrder[1:]
	}
}

func (s *ShadowServer) observed(batch *deliveryv1.NodeDeliveryBatch, items, connections uint32, duplicate bool) *deliveryv1.NodeDeliveryObservation {
	return &deliveryv1.NodeDeliveryObservation{
		ContractVersion: deliverycontract.ContractVersion,
		BatchId:         batch.BatchId, TargetNodeId: s.nodeID,
		Status:        deliveryv1.NodeObservationStatus_NODE_OBSERVATION_STATUS_OBSERVED,
		ObservedItems: items, ObservedConnections: connections,
		ObservedAt: timestamppb.Now(), Duplicate: duplicate,
	}
}

func (s *ShadowServer) rejected(batch *deliveryv1.NodeDeliveryBatch) *deliveryv1.NodeDeliveryObservation {
	return &deliveryv1.NodeDeliveryObservation{
		ContractVersion: deliverycontract.ContractVersion,
		BatchId:         batch.BatchId, TargetNodeId: s.nodeID,
		Status:     deliveryv1.NodeObservationStatus_NODE_OBSERVATION_STATUS_REJECTED,
		ErrorCode:  deliveryv1.DeliveryErrorCode_DELIVERY_ERROR_CODE_INVALID_ITEM,
		ObservedAt: timestamppb.Now(),
	}
}

func batchCounts(batch *deliveryv1.NodeDeliveryBatch) (uint32, uint32) {
	connections := 0
	for _, item := range batch.Items {
		connections += len(item.ConnectionIds)
	}
	return uint32(len(batch.Items)), uint32(connections)
}
