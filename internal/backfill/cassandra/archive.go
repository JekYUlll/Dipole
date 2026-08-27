package cassandrabackfill

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
)

const ArchiveSchemaV1 = "dipole.cassandra-message-archive.v1"

const maxArchiveEntryBytes = 16 << 20

type ArchiveManifest struct {
	SchemaVersion   string `json:"schema_version"`
	SnapshotID      string `json:"snapshot_id"`
	HighWatermarkID uint64 `json:"high_watermark_id"`
	EntryCount      uint64 `json:"entry_count"`
	EntriesSHA256   string `json:"entries_sha256"`
	DataFile        string `json:"data_file"`
}

type archiveMessage struct {
	ID              uint       `json:"id"`
	UUID            string     `json:"uuid"`
	ClientMessageID string     `json:"client_message_id"`
	ConversationKey string     `json:"conversation_key"`
	Seq             uint64     `json:"seq"`
	SenderUUID      string     `json:"sender_uuid"`
	TargetType      int8       `json:"target_type"`
	TargetUUID      string     `json:"target_uuid"`
	MessageType     int8       `json:"message_type"`
	Content         string     `json:"content"`
	FileID          string     `json:"file_id"`
	FileName        string     `json:"file_name"`
	FileSize        int64      `json:"file_size"`
	FileURL         string     `json:"file_url"`
	FileContentType string     `json:"file_content_type"`
	FileExpiresAt   *time.Time `json:"file_expires_at,omitempty"`
	SentAt          time.Time  `json:"sent_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type archiveEntry struct {
	SourceID uint64         `json:"source_id"`
	Message  archiveMessage `json:"message"`
}

type ArchiveSource struct {
	manifest ArchiveManifest
	dataPath string
}

func CreateArchive(ctx context.Context, source Source, manifestPath, snapshotID string, batchSize int) (ArchiveManifest, error) {
	manifestPath = filepath.Clean(manifestPath)
	snapshotID = strings.TrimSpace(snapshotID)
	if source == nil || snapshotID == "" || batchSize <= 0 {
		return ArchiveManifest{}, errors.New("Cassandra message archive source, snapshot ID, and positive batch size are required")
	}
	dataName := strings.TrimSuffix(filepath.Base(manifestPath), filepath.Ext(manifestPath)) + ".ndjson"
	dataPath := filepath.Join(filepath.Dir(manifestPath), dataName)
	if archivePathExists(manifestPath) || archivePathExists(dataPath) {
		return ArchiveManifest{}, errors.New("Cassandra message archive output already exists")
	}
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o750); err != nil {
		return ArchiveManifest{}, err
	}
	highWatermark, err := source.HighWatermark(ctx)
	if err != nil {
		return ArchiveManifest{}, fmt.Errorf("read Cassandra message archive high watermark: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(manifestPath), ".cassandra-message-archive-*.ndjson")
	if err != nil {
		return ArchiveManifest{}, err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	hash := sha256.New()
	encoder := json.NewEncoder(io.MultiWriter(temporary, hash))
	var count, last uint64
	for last < highWatermark {
		if err := ctx.Err(); err != nil {
			temporary.Close()
			return ArchiveManifest{}, err
		}
		items, err := source.ListAfter(ctx, last, highWatermark, batchSize)
		if err != nil {
			temporary.Close()
			return ArchiveManifest{}, fmt.Errorf("list Cassandra message archive source: %w", err)
		}
		if len(items) == 0 {
			temporary.Close()
			return ArchiveManifest{}, errors.New("Cassandra message archive source ended before high watermark")
		}
		for _, item := range items {
			if item.SourceID <= last || item.SourceID > highWatermark || strings.TrimSpace(item.Message.UUID) == "" {
				temporary.Close()
				return ArchiveManifest{}, errors.New("Cassandra message archive source order or payload is invalid")
			}
			if err := encoder.Encode(archiveEntry{SourceID: item.SourceID, Message: archiveMessageFromModel(item.Message)}); err != nil {
				temporary.Close()
				return ArchiveManifest{}, err
			}
			last = item.SourceID
			count++
		}
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return ArchiveManifest{}, err
	}
	if err := temporary.Close(); err != nil {
		return ArchiveManifest{}, err
	}
	if err := os.Rename(temporaryPath, dataPath); err != nil {
		return ArchiveManifest{}, err
	}
	manifest := ArchiveManifest{SchemaVersion: ArchiveSchemaV1, SnapshotID: snapshotID, HighWatermarkID: highWatermark, EntryCount: count, EntriesSHA256: hex.EncodeToString(hash.Sum(nil)), DataFile: dataName}
	payload, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		os.Remove(dataPath)
		return ArchiveManifest{}, err
	}
	if err := writeImmutableArchiveFile(manifestPath, append(payload, '\n')); err != nil {
		os.Remove(dataPath)
		return ArchiveManifest{}, err
	}
	return manifest, nil
}

func OpenArchive(manifestPath string) (*ArchiveSource, error) {
	payload, err := os.ReadFile(filepath.Clean(manifestPath))
	if err != nil {
		return nil, fmt.Errorf("read Cassandra message archive manifest: %w", err)
	}
	var manifest ArchiveManifest
	if err := json.Unmarshal(payload, &manifest); err != nil {
		return nil, fmt.Errorf("decode Cassandra message archive manifest: %w", err)
	}
	if manifest.SchemaVersion != ArchiveSchemaV1 || strings.TrimSpace(manifest.SnapshotID) == "" || manifest.DataFile != filepath.Base(manifest.DataFile) || len(manifest.EntriesSHA256) != 64 {
		return nil, errors.New("Cassandra message archive manifest is invalid")
	}
	source := &ArchiveSource{manifest: manifest, dataPath: filepath.Join(filepath.Dir(filepath.Clean(manifestPath)), manifest.DataFile)}
	if err := source.verify(); err != nil {
		return nil, err
	}
	return source, nil
}

func (s *ArchiveSource) HighWatermark(context.Context) (uint64, error) {
	return s.manifest.HighWatermarkID, nil
}

func (s *ArchiveSource) Descriptor(_ context.Context, highWatermark uint64) (SourceDescriptor, error) {
	if highWatermark != s.manifest.HighWatermarkID {
		return SourceDescriptor{}, errors.New("Cassandra message archive high watermark does not match job")
	}
	return SourceDescriptor{Kind: SourceKindMessageArchive, SnapshotID: s.manifest.SnapshotID, SHA256: s.manifest.EntriesSHA256}, nil
}

func (s *ArchiveSource) ListAfter(_ context.Context, after, through uint64, limit int) ([]SourceMessage, error) {
	file, err := os.Open(s.dataPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	result := make([]SourceMessage, 0, limit)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), maxArchiveEntryBytes)
	for scanner.Scan() {
		var item archiveEntry
		if err := json.Unmarshal(scanner.Bytes(), &item); err != nil {
			return nil, err
		}
		if item.SourceID > after && item.SourceID <= through {
			result = append(result, SourceMessage{SourceID: item.SourceID, Message: item.Message.model()})
			if len(result) == limit {
				break
			}
		}
	}
	return result, scanner.Err()
}

func (s *ArchiveSource) verify() error {
	file, err := os.Open(s.dataPath)
	if err != nil {
		return fmt.Errorf("open Cassandra message archive data: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	scanner := bufio.NewScanner(io.TeeReader(file, hash))
	scanner.Buffer(make([]byte, 64*1024), maxArchiveEntryBytes)
	var count, last uint64
	for scanner.Scan() {
		var item archiveEntry
		if err := json.Unmarshal(scanner.Bytes(), &item); err != nil || item.SourceID <= last || strings.TrimSpace(item.Message.UUID) == "" {
			return errors.New("Cassandra message archive entries are invalid")
		}
		last = item.SourceID
		count++
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if count != s.manifest.EntryCount || last != s.manifest.HighWatermarkID || hex.EncodeToString(hash.Sum(nil)) != strings.ToLower(s.manifest.EntriesSHA256) {
		return errors.New("Cassandra message archive integrity check failed")
	}
	return nil
}

func archiveMessageFromModel(message model.Message) archiveMessage {
	return archiveMessage{ID: message.ID, UUID: message.UUID, ClientMessageID: message.ClientMessageID, ConversationKey: message.ConversationKey, Seq: message.Seq, SenderUUID: message.SenderUUID, TargetType: message.TargetType, TargetUUID: message.TargetUUID, MessageType: message.MessageType, Content: message.Content, FileID: message.FileID, FileName: message.FileName, FileSize: message.FileSize, FileURL: message.FileURL, FileContentType: message.FileContentType, FileExpiresAt: message.FileExpiresAt, SentAt: message.SentAt, CreatedAt: message.CreatedAt, UpdatedAt: message.UpdatedAt}
}

func (m archiveMessage) model() model.Message {
	return model.Message{ID: m.ID, UUID: m.UUID, ClientMessageID: m.ClientMessageID, ConversationKey: m.ConversationKey, Seq: m.Seq, SenderUUID: m.SenderUUID, TargetType: m.TargetType, TargetUUID: m.TargetUUID, MessageType: m.MessageType, Content: m.Content, FileID: m.FileID, FileName: m.FileName, FileSize: m.FileSize, FileURL: m.FileURL, FileContentType: m.FileContentType, FileExpiresAt: m.FileExpiresAt, SentAt: m.SentAt, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt}
}

func archivePathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil || !errors.Is(err, os.ErrNotExist)
}

func writeImmutableArchiveFile(path string, payload []byte) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".cassandra-message-archive-*.json")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err == nil {
		_, err = temporary.Write(payload)
	}
	if err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}
