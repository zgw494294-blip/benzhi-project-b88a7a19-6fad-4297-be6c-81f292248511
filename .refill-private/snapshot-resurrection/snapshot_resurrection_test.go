package snapshot_resurrection_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/domain"
	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/store"
)

func TestMissingLedgerDoesNotResurrectSnapshotState(t *testing.T) {
	directory := t.TempDir()
	repository, err := store.Open(directory)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	value, err := domain.NewCase(
		"case-snapshot-resurrection",
		"派生快照恢复边界测试",
		"CN-44",
		"测试申请人",
		time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("NewCase() error = %v", err)
	}
	if _, err := repository.Create(value, "case_created", "测试工程师", nil); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := os.Remove(filepath.Join(directory, "events.jsonl")); err != nil {
		t.Fatalf("Remove(events.jsonl) error = %v", err)
	}

	reopened, err := store.Open(directory)
	if err != nil {
		return // 拒绝缺失事实源也是安全结果。
	}
	if resurrected, err := reopened.Get(value.ID); err == nil {
		t.Fatalf("TestMissingLedgerDoesNotResurrectSnapshotState: snapshot-only case was exposed: %+v", resurrected)
	}
}
