package audit_slice_alias

import (
	"testing"
	"time"

	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/analysis"
	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/coordination"
	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/domain"
	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/store"
)

func TestAuditQueryDoesNotExposeRepositorySlice(t *testing.T) {
	repository, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 25, 3, 0, 0, 0, time.UTC)
	value, err := domain.NewCase("case-audit-alias", "审计隔离", "CN-44", "申请人", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Create(value, "case_created", "原操作人", map[string]any{"title": "原始记录"}); err != nil {
		t.Fatal(err)
	}
	service := coordination.NewService(repository, analysis.NewEngine())

	entries, err := service.GetAudit(value.ID)
	if err != nil || len(entries) != 1 {
		t.Fatalf("initial audit = %#v, error = %v", entries, err)
	}
	entries[0].Details["title"] = "调用方篡改"

	again, err := service.GetAudit(value.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := again[0].Details["title"]; got != "原始记录" {
		t.Fatalf("audit history was mutated through query result: %#v", got)
	}
}
