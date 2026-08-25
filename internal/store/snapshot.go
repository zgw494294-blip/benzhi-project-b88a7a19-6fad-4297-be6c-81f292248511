package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/domain"
)

type projectionSnapshot struct {
	SchemaVersion int                                 `json:"schemaVersion"`
	LastSequence  int64                               `json:"lastSequence"`
	LastHash      string                              `json:"lastHash"`
	GeneratedAt   time.Time                           `json:"generatedAt"`
	Cases         map[string]*domain.CoordinationCase `json:"cases"`
	Idempotency   map[string]IdempotencyRecord        `json:"idempotency"`
}

func writeSnapshot(path string, snapshot projectionSnapshot) error {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".projection-*.tmp")
	if err != nil {
		return domain.WrapIntegrity("无法创建投影临时文件", err)
	}
	temporaryName := temporary.Name()
	cleanup := func() { _ = os.Remove(temporaryName) }
	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(snapshot); err != nil {
		_ = temporary.Close()
		cleanup()
		return domain.WrapIntegrity("无法编码投影快照", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		cleanup()
		return domain.WrapIntegrity("无法同步投影快照", err)
	}
	if err := temporary.Close(); err != nil {
		cleanup()
		return domain.WrapIntegrity("无法关闭投影快照", err)
	}
	if err := os.Rename(temporaryName, path); err != nil {
		cleanup()
		return domain.WrapIntegrity("无法原子替换投影快照", err)
	}
	directoryHandle, err := os.Open(directory)
	if err != nil {
		return domain.WrapIntegrity("无法打开投影目录", err)
	}
	defer directoryHandle.Close()
	if err := directoryHandle.Sync(); err != nil {
		return domain.WrapIntegrity("无法同步投影目录", err)
	}
	return nil
}

func readSnapshot(path string) (projectionSnapshot, error) {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return projectionSnapshot{}, nil
	}
	if err != nil {
		return projectionSnapshot{}, domain.WrapIntegrity("无法打开投影快照", err)
	}
	defer file.Close()
	var snapshot projectionSnapshot
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&snapshot); err != nil {
		return projectionSnapshot{}, domain.WrapIntegrity("投影快照格式无效", err)
	}
	if snapshot.SchemaVersion != schemaVersion {
		return projectionSnapshot{}, domain.NewError(domain.CodeIntegrity, "投影快照 schemaVersion 不受支持")
	}
	return snapshot, nil
}
