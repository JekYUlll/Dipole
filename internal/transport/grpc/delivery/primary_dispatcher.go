package deliverygrpc

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/JekYUlll/Dipole/internal/platform/correlation"
	deliverycontract "github.com/JekYUlll/Dipole/internal/realtime/delivery"
	deliveryv1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/delivery/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ConnectionDeliveryStatus uint8

const (
	ConnectionDeliveryStatusEnqueued ConnectionDeliveryStatus = iota + 1
	ConnectionDeliveryStatusOffline
	ConnectionDeliveryStatusBackpressured
)

type ConnectionDeliveryRequest struct {
	RecipientUserID string
	ConnectionIDs   []string
	DeliveryID      string
	EventType       string
	PayloadJSON     []byte
}

type ConnectionDeliveryOutcome struct {
	ConnectionID  string
	Status        ConnectionDeliveryStatus
	QueueDepth    int
	QueueCapacity int
}

type PrimaryDeliverySink interface {
	Enqueue(context.Context, ConnectionDeliveryRequest) ([]ConnectionDeliveryOutcome, error)
}

type primaryBatchState struct {
	fingerprint [sha256.Size]byte
	terminal    map[string]map[string]ConnectionDeliveryStatus
}

type PrimaryDispatcher struct {
	nodeID     string
	capacity   int
	retryAfter time.Duration
	sink       PrimaryDeliverySink
	mu         sync.Mutex
	batches    map[string]*primaryBatchState
	batchOrder []string
}

func NewPrimaryDispatcher(nodeID string, capacity int, retryAfter time.Duration, sink PrimaryDeliverySink) (*PrimaryDispatcher, error) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" || capacity < 1 || retryAfter <= 0 || retryAfter > time.Minute || sink == nil {
		return nil, errors.New("primary delivery dispatcher requires node, capacity, retry delay, and sink")
	}
	return &PrimaryDispatcher{
		nodeID: nodeID, capacity: capacity, retryAfter: retryAfter, sink: sink,
		batches: make(map[string]*primaryBatchState, capacity), batchOrder: make([]string, 0, capacity),
	}, nil
}

func (d *PrimaryDispatcher) DeliverNodeBatch(ctx context.Context, batch *deliveryv1.NodeDeliveryBatch) (*deliveryv1.DeliveryAck, error) {
	if d == nil {
		return nil, errors.New("primary delivery dispatcher is unavailable")
	}
	if err := deliverycontract.ValidateNodeBatch(batch); err != nil || batch.GetTargetNodeId() != d.nodeID {
		return rejectedDeliveryAck(batch), nil
	}
	ctx = correlation.WithContext(ctx, correlation.IDs{
		RequestID: batch.RequestId, TraceID: batch.TraceId, EventID: batch.SourceEventId,
	})
	fingerprint, err := nodeBatchFingerprint(batch)
	if err != nil {
		return nil, err
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	state := d.batches[batch.BatchId]
	if state != nil && state.fingerprint != fingerprint {
		return rejectedDeliveryAck(batch), nil
	}
	if state == nil {
		state = &primaryBatchState{fingerprint: fingerprint, terminal: make(map[string]map[string]ConnectionDeliveryStatus)}
		d.rememberBatch(batch.BatchId, state)
	}

	results := make([]*deliveryv1.DeliveryResult, 0, len(batch.Items))
	var pressure *deliveryv1.QueuePressure
	hasBackpressure := false
	for _, item := range batch.Items {
		terminal := state.terminal[item.DeliveryId]
		if terminal == nil {
			terminal = make(map[string]ConnectionDeliveryStatus, len(item.ConnectionIds))
			state.terminal[item.DeliveryId] = terminal
		}
		pending := make([]string, 0, len(item.ConnectionIds))
		for _, connectionID := range item.ConnectionIds {
			if _, complete := terminal[connectionID]; !complete {
				pending = append(pending, connectionID)
			}
		}
		itemBackpressured := false
		if len(pending) > 0 {
			outcomes, enqueueErr := d.sink.Enqueue(ctx, ConnectionDeliveryRequest{
				RecipientUserID: item.RecipientUserId, ConnectionIDs: pending, DeliveryID: item.DeliveryId,
				EventType: item.EventType, PayloadJSON: append([]byte(nil), item.PayloadJson...),
			})
			if enqueueErr != nil {
				return nil, fmt.Errorf("enqueue node delivery item %s: %w", item.DeliveryId, enqueueErr)
			}
			if len(outcomes) != len(pending) {
				return nil, fmt.Errorf("node delivery item %s returned incomplete connection evidence", item.DeliveryId)
			}
			for index, outcome := range outcomes {
				if outcome.ConnectionID != pending[index] {
					return nil, fmt.Errorf("node delivery item %s connection evidence drifted", item.DeliveryId)
				}
				switch outcome.Status {
				case ConnectionDeliveryStatusEnqueued, ConnectionDeliveryStatusOffline:
					terminal[outcome.ConnectionID] = outcome.Status
				case ConnectionDeliveryStatusBackpressured:
					if outcome.QueueCapacity < 1 || outcome.QueueDepth < outcome.QueueCapacity {
						return nil, fmt.Errorf("node delivery item %s has invalid queue pressure", item.DeliveryId)
					}
					itemBackpressured = true
					hasBackpressure = true
					if pressure == nil || uint32(outcome.QueueDepth) > pressure.Depth {
						pressure = &deliveryv1.QueuePressure{
							Depth: uint32(outcome.QueueDepth), Capacity: uint32(outcome.QueueCapacity),
							RetryAfterMs: uint32(d.retryAfter.Milliseconds()),
						}
					}
				default:
					return nil, fmt.Errorf("node delivery item %s has invalid connection status", item.DeliveryId)
				}
			}
		}
		accepted := uint32(0)
		for _, status := range terminal {
			if status == ConnectionDeliveryStatusEnqueued {
				accepted++
			}
		}
		result := &deliveryv1.DeliveryResult{DeliveryId: item.DeliveryId, AcceptedConnections: accepted}
		switch {
		case itemBackpressured:
			result.Status = deliveryv1.DeliveryResultStatus_DELIVERY_RESULT_STATUS_BACKPRESSURED
			result.RetryAfterMs = uint32(d.retryAfter.Milliseconds())
			result.ErrorCode = deliveryv1.DeliveryErrorCode_DELIVERY_ERROR_CODE_QUEUE_FULL
		case accepted > 0:
			result.Status = deliveryv1.DeliveryResultStatus_DELIVERY_RESULT_STATUS_ENQUEUED
		default:
			result.Status = deliveryv1.DeliveryResultStatus_DELIVERY_RESULT_STATUS_OFFLINE
		}
		results = append(results, result)
	}
	status := deliveryv1.DeliveryAckStatus_DELIVERY_ACK_STATUS_ACCEPTED
	if hasBackpressure {
		status = deliveryv1.DeliveryAckStatus_DELIVERY_ACK_STATUS_PARTIAL
	}
	return &deliveryv1.DeliveryAck{
		ContractVersion: deliverycontract.ContractVersion, BatchId: batch.BatchId, Status: status,
		Results: results, Pressure: pressure, AcknowledgedAt: timestamppb.Now(),
	}, nil
}

func (d *PrimaryDispatcher) rememberBatch(batchID string, state *primaryBatchState) {
	d.batches[batchID] = state
	d.batchOrder = append(d.batchOrder, batchID)
	if len(d.batchOrder) > d.capacity {
		delete(d.batches, d.batchOrder[0])
		d.batchOrder = d.batchOrder[1:]
	}
}

func nodeBatchFingerprint(batch *deliveryv1.NodeDeliveryBatch) ([sha256.Size]byte, error) {
	encoded, err := (proto.MarshalOptions{Deterministic: true}).Marshal(batch)
	if err != nil {
		return [sha256.Size]byte{}, fmt.Errorf("fingerprint node delivery batch: %w", err)
	}
	return sha256.Sum256(encoded), nil
}

func rejectedDeliveryAck(batch *deliveryv1.NodeDeliveryBatch) *deliveryv1.DeliveryAck {
	batchID := "invalid"
	items := []*deliveryv1.NodeDeliveryItem(nil)
	if batch != nil {
		if batch.BatchId != "" {
			batchID = batch.BatchId
		}
		items = batch.Items
	}
	results := make([]*deliveryv1.DeliveryResult, 0, len(items))
	for index, item := range items {
		deliveryID := fmt.Sprintf("invalid-%d", index)
		if item != nil && item.DeliveryId != "" {
			deliveryID = item.DeliveryId
		}
		results = append(results, &deliveryv1.DeliveryResult{
			DeliveryId: deliveryID, Status: deliveryv1.DeliveryResultStatus_DELIVERY_RESULT_STATUS_REJECTED,
			ErrorCode: deliveryv1.DeliveryErrorCode_DELIVERY_ERROR_CODE_INVALID_ITEM,
		})
	}
	if len(results) == 0 {
		results = append(results, &deliveryv1.DeliveryResult{
			DeliveryId: "invalid", Status: deliveryv1.DeliveryResultStatus_DELIVERY_RESULT_STATUS_REJECTED,
			ErrorCode: deliveryv1.DeliveryErrorCode_DELIVERY_ERROR_CODE_INVALID_ITEM,
		})
	}
	return &deliveryv1.DeliveryAck{
		ContractVersion: deliverycontract.ContractVersion, BatchId: batchID,
		Status:  deliveryv1.DeliveryAckStatus_DELIVERY_ACK_STATUS_REJECTED,
		Results: results, AcknowledgedAt: timestamppb.Now(),
	}
}
