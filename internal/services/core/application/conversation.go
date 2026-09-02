package coreapplication

import (
	"time"

	applicationPort "github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	coreconversation "github.com/JekYUlll/Dipole/internal/services/core/domain/conversation"
)

type ConversationNotifier interface {
	NotifyDirectRead(receipt coreconversation.ConversationReadReceipt)
}

type ConversationDependencies struct {
	Notifier                ConversationNotifier
	Events                  applicationPort.EventPublisher
	ProjectionWriteObserver func(string, time.Duration, error)
}

// LocalConversationApplication keeps the embedded implementation behind the
// Core service boundary while remote transports continue to use application ports.
type LocalConversationApplication struct {
	*coreconversation.ConversationService
}

func NewConversationApplication(
	repository applicationPort.ConversationStore,
	users applicationPort.UserStore,
	groups applicationPort.GroupStore,
	dependencies ConversationDependencies,
) *LocalConversationApplication {
	conversationService := coreconversation.NewConversationService(
		repository,
		users,
		groups,
		dependencies.Notifier,
		dependencies.Events,
	)
	if dependencies.ProjectionWriteObserver != nil {
		conversationService.SetProjectionWriteObserver(dependencies.ProjectionWriteObserver)
	}
	return &LocalConversationApplication{ConversationService: conversationService}
}

// WithNotifier attaches the realtime read-receipt notifier without exposing
// the domain package's private notifier contract to service composition.
func (a *LocalConversationApplication) WithNotifier(notifier ConversationNotifier) *LocalConversationApplication {
	if a != nil && a.ConversationService != nil {
		a.ConversationService.WithNotifier(notifier)
	}
	return a
}

var _ interface {
	UpdateDirectConversations(*model.Message) error
	UpdateGroupConversations(*model.Message) error
} = (*LocalConversationApplication)(nil)
