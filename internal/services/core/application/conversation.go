package coreapplication

import (
	"time"

	applicationPort "github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/service"
)

type ConversationNotifier interface {
	NotifyDirectRead(receipt service.ConversationReadReceipt)
}

type ConversationDependencies struct {
	Notifier                ConversationNotifier
	Events                  applicationPort.EventPublisher
	ProjectionWriteObserver func(string, time.Duration, error)
}

// LocalConversationApplication keeps the embedded implementation behind the
// Core service boundary while remote transports continue to use application ports.
type LocalConversationApplication struct {
	*service.ConversationService
}

func NewConversationApplication(
	repository applicationPort.ConversationStore,
	users applicationPort.UserStore,
	groups applicationPort.GroupStore,
	dependencies ConversationDependencies,
) *LocalConversationApplication {
	conversationService := service.NewConversationService(
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

var _ interface {
	UpdateDirectConversations(*model.Message) error
	UpdateGroupConversations(*model.Message) error
} = (*LocalConversationApplication)(nil)
