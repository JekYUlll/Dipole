package logger

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/config"
)

func TestBuildWriteSyncerWithFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "dipole.log")

	writeSyncer, cleanup, err := buildWriteSyncer(config.Log{
		FileEnabled: true,
		FilePath:    path,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if writeSyncer == nil {
		t.Fatalf("expected write syncer")
	}
	if cleanup != nil {
		defer cleanup()
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected log file to exist, got %v", err)
	}
}

func TestBuildWriteSyncerRejectsEmptyPath(t *testing.T) {
	t.Parallel()

	_, _, err := buildWriteSyncer(config.Log{
		FileEnabled: true,
		FilePath:    "   ",
	})
	if err == nil {
		t.Fatalf("expected error for empty file path")
	}
}

func TestDatedLogPath(t *testing.T) {
	t.Parallel()

	got := datedLogPath("logs/dipole.log", time.Date(2026, 4, 13, 10, 0, 0, 0, time.UTC))
	want := filepath.Join("logs", "dipole-2026-04-13.log")
	if got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestDailyRotatingWriterRotatesAcrossDays(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "dipole.log")
	currentTime := time.Date(2026, 4, 13, 10, 0, 0, 0, time.UTC)

	writer, err := newDailyRotatingWriter(path, func() time.Time {
		return currentTime
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	defer func() {
		_ = writer.Close()
	}()

	if _, err := writer.Write([]byte("day-one\n")); err != nil {
		t.Fatalf("expected first write to succeed, got %v", err)
	}

	currentTime = currentTime.Add(24 * time.Hour)
	if _, err := writer.Write([]byte("day-two\n")); err != nil {
		t.Fatalf("expected second write to succeed, got %v", err)
	}

	firstDayPath := filepath.Join(dir, "dipole-2026-04-13.log")
	secondDayPath := filepath.Join(dir, "dipole-2026-04-14.log")

	firstDayContent, err := os.ReadFile(firstDayPath)
	if err != nil {
		t.Fatalf("expected first day log file, got %v", err)
	}
	secondDayContent, err := os.ReadFile(secondDayPath)
	if err != nil {
		t.Fatalf("expected second day log file, got %v", err)
	}

	if string(firstDayContent) != "day-one\n" {
		t.Fatalf("unexpected first day content: %q", string(firstDayContent))
	}
	if string(secondDayContent) != "day-two\n" {
		t.Fatalf("unexpected second day content: %q", string(secondDayContent))
	}
}
