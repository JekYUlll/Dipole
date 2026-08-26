// Package application defines transport-neutral use-case contracts.
package application

import (
	"context"
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
)

type MessageCommand interface {
	SendDirectMessage(senderUUID, targetUUID, content, clientMessageID string) (*model.Message, error)
	SendGroupMessage(senderUUID, groupUUID, content, clientMessageID string) (*model.Message, []string, error)
	SendDirectFileMessage(senderUUID, targetUUID, fileUUID, clientMessageID string) (*model.Message, error)
	SendGroupFileMessage(senderUUID, groupUUID, fileUUID, clientMessageID string) (*model.Message, []string, error)
}

type MessageQuery interface {
	ListDirectMessages(currentUserUUID, targetUUID string, beforeID uint, limit int) ([]*model.Message, error)
	ListDirectMessagesBeforeSeq(currentUserUUID, targetUUID string, beforeSeq uint64, limit int) ([]*model.Message, error)
	ListGroupMessages(currentUserUUID, groupUUID string, beforeID uint, limit int) ([]*model.Message, error)
	ListGroupMessagesBeforeSeq(currentUserUUID, groupUUID string, beforeSeq uint64, limit int) ([]*model.Message, error)
	ListGroupMessagesAfter(currentUserUUID, groupUUID string, afterID uint, limit int) ([]*model.Message, error)
	ListGroupMessagesAfterSeq(currentUserUUID, groupUUID string, afterSeq uint64, limit int) ([]*model.Message, error)
	ListOfflineMessages(currentUserUUID string, afterID uint, limit int) ([]*model.Message, error)
}

type MessageApplication interface {
	MessageCommand
	MessageQuery
}

type SyncPage struct {
	Items   []*model.SyncMessage
	NextSeq uint64
	HasMore bool
}

type SyncApplication interface {
	List(userUUID string, afterSeq uint64, limit int) (*SyncPage, error)
	GetCheckpoint(userUUID, deviceID string) (*model.DeviceSyncCheckpoint, error)
	AdvanceCheckpoint(userUUID, deviceID string, syncSeq uint64) (*model.DeviceSyncCheckpoint, error)
	ListGroupCheckpoints(userUUID, deviceID string, groupUUIDs []string) ([]*model.GroupSyncCheckpoint, error)
	AdvanceGroupCheckpoint(userUUID, deviceID, groupUUID string, messageSeq uint64) (*model.GroupSyncCheckpoint, error)
}

type SearchApplication interface {
	Search(principal, text string, limit int) ([]*model.MessageSearchDocument, error)
}

type CoreCapability interface {
	GetUserByUUID(userUUID string) (*model.User, error)
	CanSendDirectMessage(userUUID, friendUUID string) (bool, error)
	GetGroupByUUID(groupUUID string) (*model.Group, error)
	GetGroupMember(groupUUID, userUUID string) (*model.GroupMember, error)
	ListGroupMembers(groupUUID string) ([]*model.GroupMember, error)
	GetOwnedFile(uploaderUUID, fileUUID string) (*model.UploadedFile, error)
	ListSearchConversationKeys(userUUID string) ([]string, error)
}

type AICallLogStore interface {
	Begin(log *model.AICallLog) (bool, error)
	MarkSucceeded(triggerMessageUUID, responseMessageUUID string, promptTokens, completionTokens, totalTokens int, latencyMS int64) error
	MarkFailed(triggerMessageUUID, errorMessage string, latencyMS int64) error
}

type AdminOverviewCounts struct {
	UserTotal                      int64
	AdminUserTotal                 int64
	DisabledUserTotal              int64
	GroupTotal                     int64
	DismissedGroupTotal            int64
	MessageTotal                   int64
	ConversationTotal              int64
	ContactTotal                   int64
	PendingContactApplicationTotal int64
}

type AdminOverviewStore interface {
	OverviewCounts() (*AdminOverviewCounts, error)
}

type FileMetadataStore interface {
	Create(file *model.UploadedFile) error
	GetByUUID(uuid string) (*model.UploadedFile, error)
}

type MessageOutboxBuilder func(message *model.Message) (*model.OutboxEvent, error)

type MessageStore interface {
	CreateWithSync(message *model.Message, recipientUUIDs []string) error
	StoreWithOutboxAndSync(message *model.Message, buildOutbox MessageOutboxBuilder, recipientUUIDs []string) error
	EnsureOutbox(event *model.OutboxEvent) error
	EnsureSyncInbox(message *model.Message, recipientUUIDs []string) error
	GetByUUID(uuid string) (*model.Message, error)
	GetBySenderAndClientMessageID(senderUUID, clientMessageID string) (*model.Message, error)
	HasConversationMessages(conversationKey string) (bool, error)
	ListByConversationKey(conversationKey string, beforeID uint, limit int) ([]*model.Message, error)
	ListByConversationKeyAfter(conversationKey string, afterID uint, limit int) ([]*model.Message, error)
	ListByConversationSeqBefore(conversationKey string, beforeSeq uint64, limit int) ([]*model.Message, error)
	ListByConversationSeqAfter(conversationKey string, afterSeq uint64, limit int) ([]*model.Message, error)
	ListOfflineByUserUUID(userUUID string, afterID uint, limit int) ([]*model.Message, error)
	FindLatestAccessibleFileMessage(fileUUID, userUUID string) (*model.Message, error)
}

type SearchIndex interface {
	Apply(mutation *model.MessageSearchMutation) error
	Search(query model.MessageSearchQuery) ([]*model.MessageSearchDocument, error)
}

type SyncStore interface {
	ListByUserAfter(userUUID string, afterSeq uint64, limit int) ([]*model.SyncMessage, error)
	GetDeviceCheckpoint(userUUID, deviceID string) (*model.DeviceSyncCheckpoint, error)
	GetLatestUserSyncSequence(userUUID string) (uint64, error)
	AdvanceDeviceSyncCheckpoint(userUUID, deviceID string, syncSeq uint64) error
	ListGroupSyncCheckpoints(userUUID, deviceID string, groupUUIDs []string) ([]*model.GroupSyncCheckpoint, error)
	GetGroupSyncState(groupUUID string) (*model.GroupSyncState, error)
	AdvanceDeviceGroupSyncCheckpoint(userUUID, deviceID, groupUUID string, messageSeq uint64) error
}

type UserStore interface {
	Create(user *model.User) error
	UpsertAssistant(user *model.User) error
	GetByUUID(uuid string) (*model.User, error)
	GetByTelephone(telephone string) (*model.User, error)
	Update(user *model.User) error
	SearchActive(keyword, excludeUUID string, limit int) ([]*model.User, error)
	List(keyword string, status *int8, limit int) ([]*model.User, error)
	ListByUUIDs(uuids []string) ([]*model.User, error)
}

type ContactStore interface {
	AreFriends(userUUID, friendUUID string) (bool, error)
	CanSendDirectMessage(userUUID, friendUUID string) (bool, error)
	CreateFriendship(userOneUUID, userTwoUUID string) error
	DeleteFriendship(userOneUUID, userTwoUUID string) error
	ListFriends(userUUID string) ([]*model.Contact, error)
	GetContact(userUUID, friendUUID string) (*model.Contact, error)
	UpdateContact(contact *model.Contact) error
	CreateApplication(contactApplication *model.ContactApplication) error
	GetApplicationByPair(applicantUUID, targetUUID string) (*model.ContactApplication, error)
	GetApplicationByID(id uint) (*model.ContactApplication, error)
	UpdateApplication(contactApplication *model.ContactApplication) error
	ListIncomingApplications(userUUID string) ([]*model.ContactApplication, error)
	ListOutgoingApplications(userUUID string) ([]*model.ContactApplication, error)
}

type GroupStore interface {
	Create(group *model.Group, members []*model.GroupMember) error
	GetByUUID(groupUUID string) (*model.Group, error)
	GetMember(groupUUID, userUUID string) (*model.GroupMember, error)
	ListMembers(groupUUID string) ([]*model.GroupMember, error)
	AddMembers(groupUUID string, members []*model.GroupMember) error
	Update(group *model.Group) error
	RemoveMembers(groupUUID string, memberUUIDs []string) error
	RemoveMember(groupUUID, userUUID string) error
}

type ConversationStore interface {
	UpsertDirectMessage(userUUID, targetUUID string, message *model.Message, unreadIncrement int) error
	UpsertGroupMessage(userUUID, groupUUID string, message *model.Message, unreadIncrement int) error
	ListByUserUUID(userUUID string, limit int) ([]*model.Conversation, error)
	ListSearchConversationKeys(userUUID string) ([]string, error)
	GetByUserAndConversationKey(userUUID, conversationKey string) (*model.Conversation, error)
	InitGroupConversation(userUUID, groupUUID, conversationKey string, createdAt time.Time) error
	UpdateRemarkByConversationKey(userUUID, conversationKey, remark string) error
	MarkReadThroughByConversationKey(userUUID, conversationKey string, readThroughSeq uint64) error
}

type OutboxRelayStore interface {
	ClaimPendingBatch(limit int, now time.Time, lease time.Duration) ([]*model.OutboxEvent, error)
	MarkPublished(id uint, publishedAt time.Time) error
	MarkRetry(id uint, retryCount int, nextRetryAt time.Time, lastErr error) error
	DecodeHeaders(event *model.OutboxEvent) (map[string]string, error)
}

type EventPublisher interface {
	PublishJSON(ctx context.Context, topic string, key string, payload any, headers map[string]string) error
	PublishEvent(ctx context.Context, topic string, key string, eventType string, payload any, headers map[string]string) error
}
