package audit_filter_fallback

import (
	"testing"
	"time"

	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/coordination"
	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/domain"
	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/store"
)

type fallbackRepository struct {
	value   *domain.CoordinationCase
	entries []domain.AuditEntry
}

func (r *fallbackRepository) Create(value *domain.CoordinationCase, action, actor string, details map[string]any) (*domain.CoordinationCase, error) {
	return value, nil
}

func (r *fallbackRepository) Update(caseID string, expectedVersion int64, action, actor string, details map[string]any, idem *store.IdempotencyRequest, mutate store.Mutation) (*domain.CoordinationCase, bool, error) {
	return r.value, false, nil
}

func (r *fallbackRepository) Get(caseID string) (*domain.CoordinationCase, error) {
	if r.value == nil || r.value.ID != caseID {
		return nil, domain.NewError(domain.CodeNotFound, "协调案不存在")
	}
	return r.value, nil
}

func (r *fallbackRepository) Audit(caseID string) ([]domain.AuditEntry, error) {
	if r.value == nil || r.value.ID != caseID {
		return nil, domain.NewError(domain.CodeNotFound, "协调案不存在")
	}
	return append([]domain.AuditEntry(nil), r.entries...), nil
}

func TestFilteredAuditFallbackHonorsFilter(t *testing.T) {
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	repository := &fallbackRepository{
		value: &domain.CoordinationCase{ID: "case-fallback"},
		entries: []domain.AuditEntry{
			{Sequence: 1, CaseID: "case-fallback", Action: "case_created", Actor: "工程师", CaseVersion: 1, OccurredAt: now},
			{Sequence: 2, CaseID: "case-fallback", Action: "proposal_revised", Actor: "工程师", CaseVersion: 2, OccurredAt: now.Add(time.Minute)},
			{Sequence: 3, CaseID: "case-fallback", Action: "review_decided", Actor: "复核员", CaseVersion: 3, OccurredAt: now.Add(2 * time.Minute)},
		},
	}
	service := coordination.NewServiceWithDependencies(repository, nil, func() time.Time { return now }, func(prefix string) string { return prefix + "-id" })

	page, err := service.GetAuditFiltered("case-fallback", store.AuditFilter{Action: "review_decided", Limit: 1})
	if err != nil {
		t.Fatalf("GetAuditFiltered() error = %v", err)
	}
	if len(page.Entries) != 1 || page.Entries[0].Action != "review_decided" {
		t.Fatalf("filtered fallback entries = %+v", page.Entries)
	}
}
