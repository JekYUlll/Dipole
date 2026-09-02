package archive

import (
	"context"
	"io"
	"time"
)

type ObjectVersion struct {
	Bucket    string `json:"bucket"`
	ObjectKey string `json:"object_key"`
	VersionID string `json:"version_id"`
	ETag      string `json:"etag"`
}

type VersionedObjectStore interface {
	ValidateRetention(context.Context, time.Duration) error
	PutFile(context.Context, string, string, time.Time) (ObjectVersion, error)
	GetVersion(context.Context, ObjectVersion, io.Writer) error
}
