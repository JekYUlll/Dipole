package cassandrabackfill

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	platformarchive "github.com/JekYUlll/Dipole/internal/platform/archive"
)

const ArchiveReceiptSchemaV1 = "dipole.cassandra-message-archive-receipt.v1"

var archiveSnapshotIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

type ArchiveReceipt struct {
	SchemaVersion   string                        `json:"schema_version"`
	SnapshotID      string                        `json:"snapshot_id"`
	HighWatermarkID uint64                        `json:"high_watermark_id"`
	EntriesSHA256   string                        `json:"entries_sha256"`
	RetainUntil     time.Time                     `json:"retain_until"`
	Manifest        platformarchive.ObjectVersion `json:"manifest"`
	Data            platformarchive.ObjectVersion `json:"data"`
}

func PublishArchive(ctx context.Context, store platformarchive.VersionedObjectStore, manifestPath, prefix string, now time.Time, retention time.Duration) (ArchiveReceipt, error) {
	if store == nil || retention <= 0 {
		return ArchiveReceipt{}, errors.New("versioned archive store and positive retention are required")
	}
	archive, err := OpenArchive(manifestPath)
	if err != nil {
		return ArchiveReceipt{}, err
	}
	if !archiveSnapshotIDPattern.MatchString(archive.manifest.SnapshotID) {
		return ArchiveReceipt{}, errors.New("Cassandra message archive snapshot ID is unsafe for object storage")
	}
	if err := store.ValidateRetention(ctx, retention); err != nil {
		return ArchiveReceipt{}, fmt.Errorf("validate Cassandra message archive retention: %w", err)
	}
	retainUntil := now.UTC().Add(retention)
	base := path.Join(strings.Trim(strings.TrimSpace(prefix), "/"), archive.manifest.SnapshotID)
	data, err := store.PutFile(ctx, path.Join(base, archive.manifest.DataFile), archive.dataPath, retainUntil)
	if err != nil {
		return ArchiveReceipt{}, fmt.Errorf("publish Cassandra message archive data: %w", err)
	}
	manifest, err := store.PutFile(ctx, path.Join(base, filepath.Base(manifestPath)), filepath.Clean(manifestPath), retainUntil)
	if err != nil {
		return ArchiveReceipt{}, fmt.Errorf("publish Cassandra message archive manifest: %w", err)
	}
	if data.VersionID == "" || manifest.VersionID == "" {
		return ArchiveReceipt{}, errors.New("Cassandra message archive object store did not return version IDs")
	}
	return ArchiveReceipt{SchemaVersion: ArchiveReceiptSchemaV1, SnapshotID: archive.manifest.SnapshotID, HighWatermarkID: archive.manifest.HighWatermarkID, EntriesSHA256: archive.manifest.EntriesSHA256, RetainUntil: retainUntil, Manifest: manifest, Data: data}, nil
}

func RestoreArchive(ctx context.Context, store platformarchive.VersionedObjectStore, receipt ArchiveReceipt, destination string) (string, error) {
	if store == nil || receipt.SchemaVersion != ArchiveReceiptSchemaV1 || receipt.Manifest.VersionID == "" || receipt.Data.VersionID == "" {
		return "", errors.New("Cassandra message archive receipt is invalid")
	}
	destination = filepath.Clean(destination)
	if err := os.MkdirAll(destination, 0o750); err != nil {
		return "", err
	}
	manifestPath := filepath.Join(destination, path.Base(receipt.Manifest.ObjectKey))
	dataPath := filepath.Join(destination, path.Base(receipt.Data.ObjectKey))
	if archivePathExists(manifestPath) || archivePathExists(dataPath) {
		return "", errors.New("Cassandra message archive restore output already exists")
	}
	if err := restoreArchiveObjectVersion(ctx, store, receipt.Data, dataPath); err != nil {
		return "", err
	}
	if err := restoreArchiveObjectVersion(ctx, store, receipt.Manifest, manifestPath); err != nil {
		os.Remove(dataPath)
		return "", err
	}
	archive, err := OpenArchive(manifestPath)
	if err != nil {
		os.Remove(dataPath)
		os.Remove(manifestPath)
		return "", err
	}
	if archive.manifest.SnapshotID != receipt.SnapshotID || archive.manifest.HighWatermarkID != receipt.HighWatermarkID || archive.manifest.EntriesSHA256 != receipt.EntriesSHA256 {
		os.Remove(dataPath)
		os.Remove(manifestPath)
		return "", errors.New("restored Cassandra message archive does not match receipt")
	}
	return manifestPath, nil
}

func WriteArchiveReceipt(filePath string, receipt ArchiveReceipt) error {
	filePath = filepath.Clean(filePath)
	if archivePathExists(filePath) {
		return errors.New("Cassandra message archive receipt already exists")
	}
	payload, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}
	return writeImmutableArchiveFile(filePath, append(payload, '\n'))
}

func ReadArchiveReceipt(filePath string) (ArchiveReceipt, error) {
	payload, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		return ArchiveReceipt{}, err
	}
	var receipt ArchiveReceipt
	if err := json.Unmarshal(payload, &receipt); err != nil {
		return ArchiveReceipt{}, err
	}
	return receipt, nil
}

func restoreArchiveObjectVersion(ctx context.Context, store platformarchive.VersionedObjectStore, version platformarchive.ObjectVersion, destination string) error {
	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	err = store.GetVersion(ctx, version, file)
	if syncErr := file.Sync(); err == nil {
		err = syncErr
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(destination)
	}
	return err
}
