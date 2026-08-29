package application

import "errors"

// File errors shared by the Core file capability and Message file validation.
// Keeping them at the application boundary avoids a service-domain import cycle.
var (
	ErrFileStorageUnavailable = errors.New("file storage is unavailable")
	ErrFileNotFound           = errors.New("file not found")
	ErrFilePermissionDenied   = errors.New("file permission denied")
)
