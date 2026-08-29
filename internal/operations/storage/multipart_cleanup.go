package storageops

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
)

type MultipartUploadCandidate struct {
	ObjectKey string    `json:"object_key"`
	UploadID  string    `json:"upload_id"`
	Initiated time.Time `json:"initiated_at"`
	Status    string    `json:"status"`
	Error     string    `json:"error,omitempty"`
}

type MultipartCleanupReport struct {
	Bucket     string                     `json:"bucket"`
	Prefix     string                     `json:"prefix"`
	Cutoff     time.Time                  `json:"cutoff"`
	Execute    bool                       `json:"execute"`
	Scanned    int                        `json:"scanned"`
	Selected   int                        `json:"selected"`
	Aborted    int                        `json:"aborted"`
	Failed     int                        `json:"failed"`
	Candidates []MultipartUploadCandidate `json:"candidates"`
}

type MultipartClient interface {
	ListIncompleteUploads(ctx context.Context, bucketName, objectPrefix string, recursive bool) <-chan minio.ObjectMultipartInfo
	AbortMultipartUpload(ctx context.Context, bucketName, objectName, uploadID string) error
}

func RunMultipartCleanup(ctx context.Context, client MultipartClient, bucket, prefix string, cutoff time.Time, execute bool) MultipartCleanupReport {
	report := MultipartCleanupReport{
		Bucket: bucket, Prefix: prefix, Cutoff: cutoff.UTC(), Execute: execute,
		Candidates: make([]MultipartUploadCandidate, 0),
	}
	for upload := range client.ListIncompleteUploads(ctx, bucket, prefix, true) {
		report.Scanned++
		if upload.Err != nil || upload.Initiated.IsZero() || !upload.Initiated.Before(cutoff) {
			continue
		}
		report.Selected++
		candidate := MultipartUploadCandidate{
			ObjectKey: upload.Key, UploadID: upload.UploadID, Initiated: upload.Initiated.UTC(), Status: "eligible",
		}
		if execute {
			if err := client.AbortMultipartUpload(ctx, bucket, upload.Key, upload.UploadID); err != nil {
				report.Failed++
				candidate.Status = "failed"
				candidate.Error = err.Error()
			} else {
				report.Aborted++
				candidate.Status = "aborted"
			}
		}
		report.Candidates = append(report.Candidates, candidate)
	}
	sort.Slice(report.Candidates, func(i, j int) bool {
		return report.Candidates[i].Initiated.Before(report.Candidates[j].Initiated)
	})
	return report
}

func NormalizePrefix(prefix string) string {
	return strings.Trim(strings.TrimSpace(prefix), "/") + "/"
}
