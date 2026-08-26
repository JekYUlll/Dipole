// Package application defines transport-neutral use-case contracts.
package application

import (
	"context"

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
	ListGroupMessages(currentUserUUID, groupUUID string, beforeID uint, limit int) ([]*model.Message, error)
	ListGroupMessagesAfter(currentUserUUID, groupUUID string, afterID uint, limit int) ([]*model.Message, error)
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
}

type CoreCapability interface {
	GetUserByUUID(userUUID string) (*model.User, error)
	CanSendDirectMessage(userUUID, friendUUID string) (bool, error)
	GetGroupByUUID(groupUUID string) (*model.Group, error)
	GetGroupMember(groupUUID, userUUID string) (*model.GroupMember, error)
	ListGroupMembers(groupUUID string) ([]*model.GroupMember, error)
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

type EventPublisher interface {
	PublishJSON(ctx context.Context, topic string, key string, payload any, headers map[string]string) error
	PublishEvent(ctx context.Context, topic string, key string, eventType string, payload any, headers map[string]string) error
}
