package ledgerrotationstalehandle_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/domain"
	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/store"
)

func TestLedgerRotationDoesNotReportDurableSuccess(t *testing.T) {
	directory := t.TempDir()
	repository, err := store.Open(directory)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	now := time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC)
	value, err := domain.NewCase("case-ledger-rotation", "账本轮转测试", "CN-44", "申请人", now)
	if err != nil {
		t.Fatalf("NewCase() error = %v", err)
	}
	value, err = repository.Create(value, "case_created", "工程师", nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	ledgerPath := filepath.Join(directory, "events.jsonl")
	if err := os.Rename(ledgerPath, filepath.Join(directory, "events.rotated.jsonl")); err != nil {
		t.Fatalf("Rename() error = %v", err)
	}
	replacement, err := os.OpenFile(ledgerPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("OpenFile(replacement) error = %v", err)
	}
	if err := replacement.Close(); err != nil {
		t.Fatalf("Close(replacement) error = %v", err)
	}

	_, _, err = repository.Update(value.ID, value.Version, "proposal_revised", "工程师", nil, nil, func(candidate *domain.CoordinationCase) error {
		return candidate.ReplaceProposal(domain.TransmitterProposal{
			FrequencyHz: 100_000_000, BandwidthHz: 200_000, EIRPDbm: 5,
			AntennaGainDbi: 2, AntennaHeightM: 30, Latitude: 23.1, Longitude: 113.3,
			EmissionClass: "F3E", Rationale: "轮转后的候选修订",
		}, now.Add(time.Minute))
	})
	if err == nil {
		t.Fatal("账本路径已被替换，但 Update 仍向失效句柄写入并报告成功")
	}
}
