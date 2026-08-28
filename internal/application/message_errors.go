package application

import "errors"

var (
	ErrMessageTargetRequired         = errors.New("message target is required")
	ErrMessageContentRequired        = errors.New("message content is required")
	ErrMessageContentTooLong         = errors.New("message content is too long")
	ErrMessageTargetUnavailable      = errors.New("message target is unavailable")
	ErrMessageTargetNotFound         = errors.New("message target not found")
	ErrMessageFriendRequired         = errors.New("direct message requires friendship")
	ErrMessageGroupForbidden         = errors.New("group message requires membership")
	ErrMessageFileRequired           = errors.New("message file is required")
	ErrMessageFileUnavailable        = errors.New("message file is unavailable")
	ErrMessageIdempotencyConflict    = errors.New("message idempotency key conflicts with an existing target")
	ErrMessageClientMessageIDInvalid = errors.New("message client message ID is invalid")
)
