package coreconversation

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/JekYUlll/Dipole/internal/model"
	platformEvents "github.com/JekYUlll/Dipole/internal/platform/events"
)

var ErrReadReceiptTargetMismatch = errors.New("conversation read event must target a direct conversation")

func DecodeReadReceipt(eventType string, raw json.RawMessage) (ConversationReadReceipt, error) {
	if err := platformEvents.RequireType(eventType, "conversation.direct.read"); err != nil {
		return ConversationReadReceipt{}, err
	}
	var payload ConversationReadReceipt
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ConversationReadReceipt{}, fmt.Errorf("decode Conversation read payload: %w", err)
	}
	if payload.TargetType != model.MessageTargetDirect {
		return ConversationReadReceipt{}, fmt.Errorf("%w: target_type=%d", ErrReadReceiptTargetMismatch, payload.TargetType)
	}
	return payload, nil
}
