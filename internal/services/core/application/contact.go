package coreapplication

import (
	"time"

	applicationPort "github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	corecontact "github.com/JekYUlll/Dipole/internal/services/core/domain/contact"
)

type ContactNotifier interface {
	NotifyFriendDeleted(userUUID, friendUUID string, occurredAt time.Time)
}

type ContactDependencies struct {
	Notifier        ContactNotifier
	Events          applicationPort.EventPublisher
	SystemMessenger interface {
		SendSystemDirectMessage(senderUUID, targetUUID, content string) (*model.Message, error)
	}
}

// LocalContactApplication keeps Core contact use cases behind the service boundary.
type LocalContactApplication struct {
	*corecontact.ContactService
}

func NewContactApplication(
	repository applicationPort.ContactStore,
	users applicationPort.UserStore,
	dependencies ContactDependencies,
) *LocalContactApplication {
	contactService := corecontact.NewContactService(repository, users).
		WithNotifier(dependencies.Notifier).
		WithEvents(dependencies.Events).
		WithSystemMessenger(dependencies.SystemMessenger)
	return &LocalContactApplication{ContactService: contactService}
}
