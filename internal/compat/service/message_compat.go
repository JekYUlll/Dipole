package service

import (
	applicationPort "github.com/JekYUlll/Dipole/internal/application"
	messagedomain "github.com/JekYUlll/Dipole/internal/services/message/domain"
)

type MessageService = messagedomain.MessageService

var (
	ErrMessageTargetRequired      = applicationPort.ErrMessageTargetRequired
	ErrMessageContentRequired     = applicationPort.ErrMessageContentRequired
	ErrMessageContentTooLong      = applicationPort.ErrMessageContentTooLong
	ErrMessageTargetUnavailable   = applicationPort.ErrMessageTargetUnavailable
	ErrMessageTargetNotFound      = applicationPort.ErrMessageTargetNotFound
	ErrMessageFriendRequired      = applicationPort.ErrMessageFriendRequired
	ErrMessageGroupForbidden      = applicationPort.ErrMessageGroupForbidden
	ErrMessageFileRequired        = applicationPort.ErrMessageFileRequired
	ErrMessageFileUnavailable     = applicationPort.ErrMessageFileUnavailable
	ErrMessageIdempotencyConflict = applicationPort.ErrMessageIdempotencyConflict
)

func NewMessageService(repo messagedomain.MessageRepository, userFinder messagedomain.MessageUserFinder, friendChecker messagedomain.FriendshipChecker, groupChecker messagedomain.GroupMessageChecker, fileFinder messagedomain.MessageFileFinder, events messagedomain.EventPublisher, hotGroups messagedomain.HotGroupObserver) *MessageService {
	return messagedomain.NewMessageService(repo, userFinder, friendChecker, groupChecker, fileFinder, events, hotGroups)
}

func NewMessageServiceWithCore(repo messagedomain.MessageRepository, core applicationPort.CoreCapability, fileFinder messagedomain.MessageFileFinder, events messagedomain.EventPublisher, hotGroups messagedomain.HotGroupObserver) *MessageService {
	return messagedomain.NewMessageServiceWithCore(repo, core, fileFinder, events, hotGroups)
}
