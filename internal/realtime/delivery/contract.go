// Package delivery defines the stable handoff to the realtime data plane.
package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/platform/correlation"
	deliveryv1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/delivery/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	ContractVersion = "v1"
	MaxBatchItems   = 4096
)

type Dispatcher interface {
	Deliver(context.Context, *deliveryv1.DeliveryEnvelope) (*deliveryv1.DeliveryAck, error)
}

type UserEventSender interface {
	SendEventToUserContext(context.Context, string, string, any) int
}

func ValidateEnvelope(batch *deliveryv1.DeliveryEnvelope) error {
	if batch == nil {
		return errors.New("delivery batch is required")
	}
	if batch.ContractVersion != ContractVersion {
		return fmt.Errorf("delivery contract version %q is unsupported", batch.ContractVersion)
	}
	if strings.TrimSpace(batch.BatchId) == "" || strings.TrimSpace(batch.SourceEventId) == "" || strings.TrimSpace(batch.SourceTopic) == "" {
		return errors.New("delivery batch identity and source are required")
	}
	if batch.SourcePartition < 0 || batch.SourceOffset < 0 || !validTimestamp(batch.CreatedAt) {
		return errors.New("delivery source coordinates and created_at are invalid")
	}
	if len(batch.Items) == 0 || len(batch.Items) > MaxBatchItems {
		return fmt.Errorf("delivery batch item count must be between 1 and %d", MaxBatchItems)
	}
	seen := make(map[string]struct{}, len(batch.Items))
	for index, item := range batch.Items {
		if item == nil {
			return fmt.Errorf("delivery item %d is required", index)
		}
		id := strings.TrimSpace(item.DeliveryId)
		if id == "" || strings.TrimSpace(item.RecipientUserId) == "" || strings.TrimSpace(item.EventType) == "" || strings.TrimSpace(item.OrderingKey) == "" {
			return fmt.Errorf("delivery item %d identity, recipient, event type, and ordering key are required", index)
		}
		if _, exists := seen[id]; exists {
			return fmt.Errorf("delivery id %q is duplicated", id)
		}
		seen[id] = struct{}{}
		if !json.Valid(item.PayloadJson) {
			return fmt.Errorf("delivery item %q payload_json is invalid", id)
		}
		if !validMode(item.Mode) {
			return fmt.Errorf("delivery item %q mode is required", id)
		}
	}
	return nil
}

func ValidateNodeBatch(batch *deliveryv1.NodeDeliveryBatch) error {
	if batch == nil {
		return errors.New("node delivery batch is required")
	}
	if batch.ContractVersion != ContractVersion {
		return fmt.Errorf("delivery contract version %q is unsupported", batch.ContractVersion)
	}
	if strings.TrimSpace(batch.BatchId) == "" || strings.TrimSpace(batch.TargetNodeId) == "" || strings.TrimSpace(batch.SourceEventId) == "" {
		return errors.New("node delivery batch identity, target node, and source event are required")
	}
	if !validTimestamp(batch.CreatedAt) {
		return errors.New("node delivery batch created_at is invalid")
	}
	if len(batch.Items) == 0 || len(batch.Items) > MaxBatchItems {
		return fmt.Errorf("node delivery batch item count must be between 1 and %d", MaxBatchItems)
	}
	seen := make(map[string]struct{}, len(batch.Items))
	for index, item := range batch.Items {
		if item == nil {
			return fmt.Errorf("node delivery item %d is required", index)
		}
		id := strings.TrimSpace(item.DeliveryId)
		if id == "" || strings.TrimSpace(item.RecipientUserId) == "" || strings.TrimSpace(item.EventType) == "" || strings.TrimSpace(item.OrderingKey) == "" {
			return fmt.Errorf("node delivery item %d identity, recipient, event type, and ordering key are required", index)
		}
		if _, exists := seen[id]; exists {
			return fmt.Errorf("node delivery id %q is duplicated", id)
		}
		seen[id] = struct{}{}
		if len(item.ConnectionIds) == 0 || !json.Valid(item.PayloadJson) {
			return fmt.Errorf("node delivery item %q connections and valid payload_json are required", id)
		}
		connections := make(map[string]struct{}, len(item.ConnectionIds))
		for _, connectionID := range item.ConnectionIds {
			connectionID = strings.TrimSpace(connectionID)
			if connectionID == "" {
				return fmt.Errorf("node delivery item %q has an empty connection id", id)
			}
			if _, exists := connections[connectionID]; exists {
				return fmt.Errorf("node delivery item %q repeats connection %q", id, connectionID)
			}
			connections[connectionID] = struct{}{}
		}
		if !validMode(item.Mode) {
			return fmt.Errorf("node delivery item %q mode is required", id)
		}
	}
	return nil
}

func ValidateAck(ack *deliveryv1.DeliveryAck) error {
	if ack == nil {
		return errors.New("delivery ack is required")
	}
	if ack.ContractVersion != ContractVersion || strings.TrimSpace(ack.BatchId) == "" || !validAckStatus(ack.Status) || !validTimestamp(ack.AcknowledgedAt) {
		return errors.New("delivery ack version, batch id, and status are required")
	}
	if len(ack.Results) == 0 {
		return errors.New("delivery ack results are required")
	}
	seen := make(map[string]struct{}, len(ack.Results))
	hasBackpressure := false
	hasNonAccepted := false
	hasAccepted := false
	for index, result := range ack.Results {
		if result == nil || strings.TrimSpace(result.DeliveryId) == "" || !validResultStatus(result.Status) {
			return fmt.Errorf("delivery result %d identity and status are required", index)
		}
		if _, exists := seen[result.DeliveryId]; exists {
			return fmt.Errorf("delivery result %q is duplicated", result.DeliveryId)
		}
		seen[result.DeliveryId] = struct{}{}
		if result.Status == deliveryv1.DeliveryResultStatus_DELIVERY_RESULT_STATUS_BACKPRESSURED {
			hasBackpressure = true
			if result.RetryAfterMs == 0 || result.ErrorCode != deliveryv1.DeliveryErrorCode_DELIVERY_ERROR_CODE_QUEUE_FULL {
				return fmt.Errorf("backpressured delivery result %q requires retry_after_ms and queue_full", result.DeliveryId)
			}
		}
		switch result.Status {
		case deliveryv1.DeliveryResultStatus_DELIVERY_RESULT_STATUS_ENQUEUED:
			if result.AcceptedConnections == 0 || result.RetryAfterMs != 0 || result.ErrorCode != deliveryv1.DeliveryErrorCode_DELIVERY_ERROR_CODE_UNSPECIFIED {
				return fmt.Errorf("enqueued delivery result %q has inconsistent evidence", result.DeliveryId)
			}
		case deliveryv1.DeliveryResultStatus_DELIVERY_RESULT_STATUS_OFFLINE:
			if result.AcceptedConnections != 0 || result.RetryAfterMs != 0 || result.ErrorCode != deliveryv1.DeliveryErrorCode_DELIVERY_ERROR_CODE_UNSPECIFIED {
				return fmt.Errorf("offline delivery result %q has inconsistent evidence", result.DeliveryId)
			}
		case deliveryv1.DeliveryResultStatus_DELIVERY_RESULT_STATUS_REJECTED:
			if result.ErrorCode != deliveryv1.DeliveryErrorCode_DELIVERY_ERROR_CODE_INVALID_ITEM {
				return fmt.Errorf("rejected delivery result %q requires invalid_item", result.DeliveryId)
			}
		case deliveryv1.DeliveryResultStatus_DELIVERY_RESULT_STATUS_FAILED:
			if result.ErrorCode != deliveryv1.DeliveryErrorCode_DELIVERY_ERROR_CODE_NODE_UNAVAILABLE && result.ErrorCode != deliveryv1.DeliveryErrorCode_DELIVERY_ERROR_CODE_INTERNAL {
				return fmt.Errorf("failed delivery result %q requires a failure error code", result.DeliveryId)
			}
		}
		if result.Status == deliveryv1.DeliveryResultStatus_DELIVERY_RESULT_STATUS_ENQUEUED || result.Status == deliveryv1.DeliveryResultStatus_DELIVERY_RESULT_STATUS_OFFLINE {
			hasAccepted = true
		} else {
			hasNonAccepted = true
		}
	}
	if hasBackpressure && (ack.Pressure == nil || ack.Pressure.Capacity == 0 || ack.Pressure.RetryAfterMs == 0 || ack.Pressure.Depth < ack.Pressure.Capacity) {
		return errors.New("backpressured delivery ack requires saturated queue pressure and retry hint")
	}
	if ack.Status == deliveryv1.DeliveryAckStatus_DELIVERY_ACK_STATUS_ACCEPTED && hasNonAccepted {
		return errors.New("accepted delivery ack contains a non-accepted result")
	}
	if ack.Status == deliveryv1.DeliveryAckStatus_DELIVERY_ACK_STATUS_PARTIAL && !hasNonAccepted {
		return errors.New("partial delivery ack requires a non-accepted result")
	}
	if ack.Status == deliveryv1.DeliveryAckStatus_DELIVERY_ACK_STATUS_REJECTED && hasAccepted {
		return errors.New("rejected delivery ack contains an accepted result")
	}
	return nil
}

func validTimestamp(value *timestamppb.Timestamp) bool {
	return value != nil && value.IsValid()
}

func validMode(value deliveryv1.DeliveryMode) bool {
	return value >= deliveryv1.DeliveryMode_DELIVERY_MODE_FULL_EVENT && value <= deliveryv1.DeliveryMode_DELIVERY_MODE_HOT_GROUP_NOTIFY
}

func validAckStatus(value deliveryv1.DeliveryAckStatus) bool {
	return value >= deliveryv1.DeliveryAckStatus_DELIVERY_ACK_STATUS_ACCEPTED && value <= deliveryv1.DeliveryAckStatus_DELIVERY_ACK_STATUS_REJECTED
}

func validResultStatus(value deliveryv1.DeliveryResultStatus) bool {
	return value >= deliveryv1.DeliveryResultStatus_DELIVERY_RESULT_STATUS_ENQUEUED && value <= deliveryv1.DeliveryResultStatus_DELIVERY_RESULT_STATUS_FAILED
}

// LegacyDispatcher lets the current Go Gateway satisfy the v1 contract while
// C++ delivery remains shadow-only.
type LegacyDispatcher struct {
	sender UserEventSender
}

func NewLegacyDispatcher(sender UserEventSender) *LegacyDispatcher {
	return &LegacyDispatcher{sender: sender}
}

func (d *LegacyDispatcher) Deliver(ctx context.Context, batch *deliveryv1.DeliveryEnvelope) (*deliveryv1.DeliveryAck, error) {
	if d == nil || d.sender == nil {
		return nil, errors.New("legacy delivery sender is required")
	}
	if err := ValidateEnvelope(batch); err != nil {
		return nil, err
	}
	ctx = correlation.WithContext(ctx, correlation.IDs{
		RequestID: batch.RequestId,
		TraceID:   batch.TraceId,
		EventID:   batch.SourceEventId,
	})
	results := make([]*deliveryv1.DeliveryResult, 0, len(batch.Items))
	for _, item := range batch.Items {
		accepted := d.sender.SendEventToUserContext(ctx, item.RecipientUserId, item.EventType, json.RawMessage(item.PayloadJson))
		status := deliveryv1.DeliveryResultStatus_DELIVERY_RESULT_STATUS_ENQUEUED
		if accepted == 0 {
			status = deliveryv1.DeliveryResultStatus_DELIVERY_RESULT_STATUS_OFFLINE
		}
		results = append(results, &deliveryv1.DeliveryResult{
			DeliveryId: item.DeliveryId, Status: status, AcceptedConnections: uint32(accepted),
		})
	}
	return &deliveryv1.DeliveryAck{
		ContractVersion: ContractVersion,
		BatchId:         batch.BatchId,
		Status:          deliveryv1.DeliveryAckStatus_DELIVERY_ACK_STATUS_ACCEPTED,
		Results:         results,
		AcknowledgedAt:  timestamppb.New(time.Now().UTC()),
	}, nil
}
