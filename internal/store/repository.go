package store

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/domain"
)

type Repository struct {
	mu           sync.RWMutex
	directory    string
	ledgerPath   string
	ledgerFile   *os.File
	snapshotPath string
	cases        map[string]*domain.CoordinationCase
	idempotency  map[string]IdempotencyRecord
	audit        map[string][]domain.AuditEntry
	lastSequence int64
	lastHash     string
}

type Mutation func(*domain.CoordinationCase) error

type AuditFilter struct {
	Action                 string
	Actor                  string
	From, To               *time.Time
	MinVersion, MaxVersion *int64
	Limit                  int
	After                  int64
}
type AuditPage struct {
	Entries              []domain.AuditEntry `json:"entries"`
	ChainValid           bool                `json:"chainValid"`
	CheckedCount         int                 `json:"checkedCount"`
	FirstInvalidSequence int64               `json:"firstInvalidSequence,omitempty"`
	NextAfter            int64               `json:"nextAfter,omitempty"`
}

func (r *Repository) Create(value *domain.CoordinationCase, action, actor string, details map[string]any) (*domain.CoordinationCase, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.cases[value.ID]; exists {
		return nil, domain.NewError(domain.CodeAlreadyExists, "协调案 ID 已存在")
	}
	copyValue := cloneCase(value)
	if err := r.commit(copyValue, action, actor, details, nil, time.Now().UTC()); err != nil {
		return nil, err
	}
	return cloneCase(copyValue), nil
}

func (r *Repository) Update(caseID string, expectedVersion int64, action, actor string, details map[string]any, idem *IdempotencyRequest, mutate Mutation) (*domain.CoordinationCase, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := validateIdempotency(idem); err != nil {
		return nil, false, err
	}
	if idem != nil {
		indexKey := idempotencyIndexKey(caseID, action, idem.Key)
		if record, exists := r.idempotency[indexKey]; exists {
			if record.Fingerprint != idem.Fingerprint {
				return nil, false, domain.NewError(domain.CodeIdempotency, "相同 idempotencyKey 已用于不同请求")
			}
			return cloneCase(record.CaseSnapshot), true, nil
		}
	}
	current, exists := r.cases[caseID]
	if !exists {
		return nil, false, domain.NewError(domain.CodeNotFound, "协调案不存在")
	}
	if err := current.EnsureVersion(expectedVersion); err != nil {
		return nil, false, err
	}
	next := cloneCase(current)
	if err := mutate(next); err != nil {
		return nil, false, err
	}
	if next.Version <= current.Version {
		return nil, false, domain.NewError(domain.CodeIntegrity, "业务变更未推进协调案版本")
	}
	var record *IdempotencyRecord
	if idem != nil {
		record = &IdempotencyRecord{Key: idem.Key, Fingerprint: idem.Fingerprint, CaseID: caseID, Action: action, CaseSnapshot: cloneCase(next), RecordedAt: time.Now().UTC()}
	}
	if err := r.commit(next, action, actor, details, record, time.Now().UTC()); err != nil {
		return nil, false, err
	}
	return cloneCase(next), false, nil
}

func (r *Repository) Get(caseID string) (*domain.CoordinationCase, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	value, exists := r.cases[caseID]
	if !exists {
		return nil, domain.NewError(domain.CodeNotFound, "协调案不存在")
	}
	return cloneCase(value), nil
}

func (r *Repository) Audit(caseID string) ([]domain.AuditEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, exists := r.cases[caseID]; !exists {
		return nil, domain.NewError(domain.CodeNotFound, "协调案不存在")
	}
	entries := r.audit[caseID]
	result := make([]domain.AuditEntry, len(entries))
	for i, entry := range entries {
		result[i] = entry
		result[i].Details = cloneDetails(entry.Details)
	}
	return result, nil
}

func (r *Repository) AuditFiltered(caseID string, filter AuditFilter) (AuditPage, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.cases[caseID]; !ok {
		return AuditPage{}, domain.NewError(domain.CodeNotFound, "协调案不存在")
	}
	all := r.audit[caseID]
	page := AuditPage{Entries: []domain.AuditEntry{}, ChainValid: true}
	if _, err := readLedger(r.ledgerPath); err != nil {
		page.ChainValid = false
	}
	var previous int64
	var previousHash string
	for _, entry := range all {
		page.CheckedCount++
		if previous > 0 && (entry.Sequence <= previous || entry.PreviousHash != previousHash) {
			page.ChainValid = false
			if page.FirstInvalidSequence == 0 {
				page.FirstInvalidSequence = entry.Sequence
			}
		}
		previous = entry.Sequence
		previousHash = entry.Hash
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	for _, entry := range all {
		if entry.Sequence <= filter.After {
			continue
		}
		if filter.Action != "" && entry.Action != filter.Action {
			continue
		}
		if filter.Actor != "" && entry.Actor != filter.Actor {
			continue
		}
		if filter.From != nil && entry.OccurredAt.Before(filter.From.UTC()) {
			continue
		}
		if filter.To != nil && entry.OccurredAt.After(filter.To.UTC()) {
			continue
		}
		if filter.MinVersion != nil && entry.CaseVersion < *filter.MinVersion {
			continue
		}
		if filter.MaxVersion != nil && entry.CaseVersion > *filter.MaxVersion {
			continue
		}
		if len(page.Entries) >= limit {
			page.NextAfter = page.Entries[len(page.Entries)-1].Sequence
			break
		}
		entry.Details = cloneDetails(entry.Details)
		page.Entries = append(page.Entries, entry)
	}
	return page, nil
}

func (r *Repository) commit(value *domain.CoordinationCase, action, actor string, details map[string]any, idem *IdempotencyRecord, occurredAt time.Time) error {
	if err := domain.ValidateCaseIntegrity(value); err != nil {
		return err
	}
	event := ledgerEvent{
		SchemaVersion: schemaVersion, Sequence: r.lastSequence + 1, CaseID: value.ID,
		Action: action, Actor: actor, CaseVersion: value.Version, OccurredAt: occurredAt.UTC(),
		Details: cloneDetails(details), Case: cloneCase(value), Idempotency: idem, PreviousHash: r.lastHash,
	}
	if err := sealEvent(&event); err != nil {
		return err
	}
	if err := r.appendLedger(event); err != nil {
		return err
	}
	r.lastSequence = event.Sequence
	r.lastHash = event.Hash
	r.applyEvent(event)
	if err := r.persistProjection(occurredAt); err != nil {
		return err
	}
	return nil
}

func (r *Repository) persistProjection(now time.Time) error {
	cases := make(map[string]*domain.CoordinationCase, len(r.cases))
	for id, value := range r.cases {
		cases[id] = cloneCase(value)
	}
	idempotency := make(map[string]IdempotencyRecord, len(r.idempotency))
	for key, value := range r.idempotency {
		idempotency[key] = cloneIdempotency(value)
	}
	return writeSnapshot(r.snapshotPath, projectionSnapshot{SchemaVersion: schemaVersion, LastSequence: r.lastSequence, LastHash: r.lastHash, GeneratedAt: now.UTC(), Cases: cases, Idempotency: idempotency})
}

func cloneCase(value *domain.CoordinationCase) *domain.CoordinationCase {
	if value == nil {
		return nil
	}
	payload, _ := json.Marshal(value)
	var result domain.CoordinationCase
	_ = json.Unmarshal(payload, &result)
	return &result
}

func cloneIdempotency(value IdempotencyRecord) IdempotencyRecord {
	value.CaseSnapshot = cloneCase(value.CaseSnapshot)
	return value
}

func cloneDetails(value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	payload, _ := json.Marshal(value)
	var result map[string]any
	_ = json.Unmarshal(payload, &result)
	return result
}
