package searchbackfill

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
)

const ArchiveSchemaV1 = "dipole.search-archive.v1"

type ArchiveManifest struct {
	SchemaVersion   string `json:"schema_version"`
	SnapshotID      string `json:"snapshot_id"`
	HighWatermarkID uint64 `json:"high_watermark_id"`
	EntryCount      uint64 `json:"entry_count"`
	EntriesSHA256   string `json:"entries_sha256"`
	DataFile        string `json:"data_file"`
}

type ArchiveSource struct {
	manifest ArchiveManifest
	dataPath string
}

func CreateArchive(ctx context.Context, source Source, manifestPath, snapshotID string, batchSize int) (ArchiveManifest, error) {
	manifestPath = filepath.Clean(manifestPath)
	snapshotID = strings.TrimSpace(snapshotID)
	if source == nil || snapshotID == "" || batchSize <= 0 {
		return ArchiveManifest{}, errors.New("Search archive source, snapshot ID, and positive batch size are required")
	}
	dataName := strings.TrimSuffix(filepath.Base(manifestPath), filepath.Ext(manifestPath)) + ".ndjson"
	dataPath := filepath.Join(filepath.Dir(manifestPath), dataName)
	if pathExists(manifestPath) || pathExists(dataPath) {
		return ArchiveManifest{}, errors.New("Search archive output already exists")
	}
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o750); err != nil {
		return ArchiveManifest{}, err
	}
	highWatermark, err := source.HighWatermark(ctx)
	if err != nil {
		return ArchiveManifest{}, fmt.Errorf("read Search archive high watermark: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(manifestPath), ".search-archive-*.ndjson")
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
			return ArchiveManifest{}, fmt.Errorf("list Search archive source: %w", err)
		}
		if len(items) == 0 {
			temporary.Close()
			return ArchiveManifest{}, errors.New("Search archive source ended before high watermark")
		}
		for _, item := range items {
			if item.SourceID <= last || item.SourceID > highWatermark || item.Mutation == nil {
				temporary.Close()
				return ArchiveManifest{}, errors.New("Search archive source order is invalid")
			}
			if err := encoder.Encode(item); err != nil {
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
	manifestFile, err := os.CreateTemp(filepath.Dir(manifestPath), ".search-archive-*.json")
	if err != nil {
		os.Remove(dataPath)
		return ArchiveManifest{}, err
	}
	temporaryManifestPath := manifestFile.Name()
	defer os.Remove(temporaryManifestPath)
	if err := manifestFile.Chmod(0o600); err == nil {
		_, err = manifestFile.Write(append(payload, '\n'))
	}
	if err == nil {
		err = manifestFile.Sync()
	}
	if closeErr := manifestFile.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(temporaryManifestPath, manifestPath)
	}
	if err != nil {
		os.Remove(dataPath)
		return ArchiveManifest{}, err
	}
	return manifest, nil
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil || !errors.Is(err, os.ErrNotExist)
}

func OpenArchive(manifestPath string) (*ArchiveSource, error) {
	payload, err := os.ReadFile(filepath.Clean(manifestPath))
	if err != nil {
		return nil, fmt.Errorf("read Search archive manifest: %w", err)
	}
	var manifest ArchiveManifest
	if err := json.Unmarshal(payload, &manifest); err != nil {
		return nil, fmt.Errorf("decode Search archive manifest: %w", err)
	}
	if manifest.SchemaVersion != ArchiveSchemaV1 || strings.TrimSpace(manifest.SnapshotID) == "" || manifest.DataFile != filepath.Base(manifest.DataFile) {
		return nil, errors.New("Search archive manifest is invalid")
	}
	source := &ArchiveSource{manifest: manifest, dataPath: filepath.Join(filepath.Dir(manifestPath), manifest.DataFile)}
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
		return SourceDescriptor{}, errors.New("Search archive high watermark does not match job")
	}
	return SourceDescriptor{Kind: SourceKindEventArchive, SnapshotID: s.manifest.SnapshotID, SHA256: s.manifest.EntriesSHA256}, nil
}

func (s *ArchiveSource) ListAfter(_ context.Context, after, through uint64, limit int) ([]SourceMutation, error) {
	file, err := os.Open(s.dataPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	result := make([]SourceMutation, 0, limit)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var item SourceMutation
		if err := json.Unmarshal(scanner.Bytes(), &item); err != nil {
			return nil, err
		}
		if item.SourceID > after && item.SourceID <= through {
			result = append(result, item)
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
		return fmt.Errorf("open Search archive data: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	tee := io.TeeReader(file, hash)
	scanner := bufio.NewScanner(tee)
	var count, last uint64
	for scanner.Scan() {
		var item SourceMutation
		if err := json.Unmarshal(scanner.Bytes(), &item); err != nil || item.SourceID <= last || item.Mutation == nil {
			return errors.New("Search archive entries are invalid")
		}
		last = item.SourceID
		count++
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if count != s.manifest.EntryCount || last != s.manifest.HighWatermarkID || hex.EncodeToString(hash.Sum(nil)) != strings.ToLower(s.manifest.EntriesSHA256) {
		return errors.New("Search archive integrity check failed")
	}
	return nil
}
