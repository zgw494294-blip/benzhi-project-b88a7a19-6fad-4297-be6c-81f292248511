package store

import (
	"os"
	"path/filepath"
	"time"

	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/domain"
)

func Open(directory string) (*Repository, error) {
	if directory == "" {
		return nil, domain.ValidationError(domain.Violation("dataDir", "required", "数据目录不能为空"))
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, domain.WrapIntegrity("无法创建数据目录", err)
	}
	repository := &Repository{
		directory:    directory,
		ledgerPath:   filepath.Join(directory, "events.jsonl"),
		snapshotPath: filepath.Join(directory, "projection.json"),
		cases:        make(map[string]*domain.CoordinationCase),
		idempotency:  make(map[string]IdempotencyRecord),
		audit:        make(map[string][]domain.AuditEntry),
	}
	if err := repository.recover(); err != nil {
		return nil, err
	}
	return repository, nil
}

func (r *Repository) recover() error {
	events, err := readLedger(r.ledgerPath)
	if err != nil {
		return err
	}
	for _, event := range events {
		if err := domain.ValidateCaseIntegrity(event.Case); err != nil {
			return domain.WrapIntegrity("账本包含无效协调案投影", err)
		}
		r.applyEvent(event)
	}
	if len(events) > 0 {
		last := events[len(events)-1]
		r.lastSequence = last.Sequence
		r.lastHash = last.Hash
	}
	snapshot, snapshotErr := readSnapshot(r.snapshotPath)
	if snapshotErr == nil && snapshot.LastSequence == r.lastSequence && snapshot.LastHash == r.lastHash {
		// 即使快照有效，账本重放仍是事实依据；这里仅确认其一致性。
		return nil
	}
	return r.persistProjection(time.Now().UTC())
}

func (r *Repository) applyEvent(event ledgerEvent) {
	r.cases[event.CaseID] = cloneCase(event.Case)
	if event.Idempotency != nil {
		key := idempotencyIndexKey(event.Idempotency.CaseID, event.Idempotency.Action, event.Idempotency.Key)
		r.idempotency[key] = cloneIdempotency(*event.Idempotency)
	}
	r.audit[event.CaseID] = append(r.audit[event.CaseID], domain.AuditEntry{
		Sequence: event.Sequence, CaseID: event.CaseID, Action: event.Action, Actor: event.Actor,
		CaseVersion: event.CaseVersion, OccurredAt: event.OccurredAt,
		Details: cloneDetails(event.Details), PreviousHash: event.PreviousHash, Hash: event.Hash,
	})
}
