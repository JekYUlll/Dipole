package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"
)

type MessageSearchMutationType string

const (
	MessageSearchMutationUpsert    MessageSearchMutationType = "upsert"
	MessageSearchMutationTombstone MessageSearchMutationType = "tombstone"
)

var ErrMessageSearchMutationConflict = errors.New("message search mutation conflict")

type MessageSearchDocument struct {
	MessageUUID     string
	ConversationKey string
	MessageSeq      uint64
	Revision        uint64
	SenderUUID      string
	MessageType     int8
	Content         string
	SentAt          time.Time
}

type MessageSearchQuery struct {
	ConversationKeys []string
	Text             string
	Limit            int
}

type MessageSearchMutation struct {
	Type        MessageSearchMutationType
	MessageUUID string
	Revision    uint64
	Document    *MessageSearchDocument
}

type MessageSearchState struct {
	MessageUUID     string     `json:"message_uuid"`
	ConversationKey string     `json:"conversation_key,omitempty"`
	MessageSeq      uint64     `json:"message_seq,omitempty"`
	Revision        uint64     `json:"revision"`
	SenderUUID      string     `json:"sender_uuid,omitempty"`
	MessageType     int8       `json:"message_type,omitempty"`
	Content         string     `json:"content,omitempty"`
	SentAt          *time.Time `json:"sent_at,omitempty"`
	Searchable      bool       `json:"searchable"`
	PayloadHash     string     `json:"payload_hash"`
}

func (m *MessageSearchMutation) State() (MessageSearchState, error) {
	if m == nil {
		return MessageSearchState{}, errors.New("search mutation is required")
	}
	messageUUID := strings.TrimSpace(m.MessageUUID)
	if messageUUID == "" || m.Revision == 0 {
		return MessageSearchState{}, errors.New("search mutation identity and revision are required")
	}
	if m.Revision > math.MaxInt64 {
		return MessageSearchState{}, errors.New("search mutation revision must fit storage long")
	}
	state := MessageSearchState{MessageUUID: messageUUID, Revision: m.Revision}
	switch m.Type {
	case MessageSearchMutationUpsert:
		if m.Document == nil {
			return MessageSearchState{}, errors.New("search upsert document is required")
		}
		documentUUID := strings.TrimSpace(m.Document.MessageUUID)
		if documentUUID != "" && documentUUID != messageUUID {
			return MessageSearchState{}, errors.New("search mutation document identity conflicts")
		}
		state.ConversationKey = strings.TrimSpace(m.Document.ConversationKey)
		state.MessageSeq = m.Document.MessageSeq
		state.SenderUUID = strings.TrimSpace(m.Document.SenderUUID)
		state.MessageType = m.Document.MessageType
		state.Content = m.Document.Content
		sentAt := m.Document.SentAt.UTC().Truncate(time.Millisecond)
		state.SentAt = &sentAt
		state.Searchable = true
		if state.ConversationKey == "" || state.MessageSeq == 0 || state.SenderUUID == "" || m.Document.SentAt.IsZero() {
			return MessageSearchState{}, errors.New("search upsert document identity is required")
		}
		if state.MessageSeq > math.MaxInt64 {
			return MessageSearchState{}, errors.New("search mutation sequence must fit storage long")
		}
	case MessageSearchMutationTombstone:
		state.Searchable = false
	default:
		return MessageSearchState{}, errors.New("search mutation type is invalid")
	}
	hashInput := state
	hashInput.PayloadHash = ""
	payload, err := json.Marshal(hashInput)
	if err != nil {
		return MessageSearchState{}, err
	}
	sum := sha256.Sum256(payload)
	state.PayloadHash = hex.EncodeToString(sum[:])
	return state, nil
}
