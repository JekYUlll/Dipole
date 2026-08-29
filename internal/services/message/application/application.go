package messageapplication

import (
	applicationPort "github.com/JekYUlll/Dipole/internal/application"
	platformHotGroup "github.com/JekYUlll/Dipole/internal/platform/hotgroup"
	messagedomain "github.com/JekYUlll/Dipole/internal/services/message/domain"
)

type HotGroupObserver interface {
	ObserveMessage(groupUUID string, memberCount int) (platformHotGroup.Status, error)
	Status(groupUUID string, memberCount int) (platformHotGroup.Status, error)
}

type Dependencies struct {
	Events                     applicationPort.EventPublisher
	HotGroups                  HotGroupObserver
	DuplicateHydrator          applicationPort.SyncMessageHydrator
	DuplicateHydrationObserver func(string)
}

// LocalApplication is the Message service's local application adapter.
// Remote transports implement the same application port at the boundary.
type LocalApplication struct {
	*messagedomain.MessageService
}

var _ applicationPort.MessageApplication = (*LocalApplication)(nil)

func New(messages applicationPort.MessageStore, core applicationPort.CoreCapability, dependencies Dependencies) *LocalApplication {
	messageService := messagedomain.NewMessageServiceWithCore(
		messages,
		core,
		nil,
		dependencies.Events,
		dependencies.HotGroups,
	)
	messageService.SetDuplicateMessageHydrator(dependencies.DuplicateHydrator, dependencies.DuplicateHydrationObserver)
	return &LocalApplication{MessageService: messageService}
}
