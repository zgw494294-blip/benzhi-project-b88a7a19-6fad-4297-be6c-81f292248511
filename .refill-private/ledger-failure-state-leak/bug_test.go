package ledger_failure_state_leak

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/domain"
	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/store"
)

func TestLedgerAppendFailureDoesNotPublishInMemoryState(t *testing.T) {
	directory := t.TempDir()
	repository, err := store.Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	createdAt := time.Date(2026, 8, 25, 3, 0, 0, 0, time.UTC)
	value, err := domain.NewCase("case-ledger-failure", "账本故障测试", "CN-44", "申请人", createdAt)
	if err != nil {
		t.Fatal(err)
	}
	value, err = repository.Create(value, "case_created", "工程师", nil)
	if err != nil {
		t.Fatal(err)
	}

	ledgerPath := filepath.Join(directory, "events.jsonl")
	if err := os.Remove(ledgerPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(ledgerPath, 0o700); err != nil {
		t.Fatal(err)
	}

	_, _, err = repository.Update(value.ID, value.Version, "proposal_revised", "工程师", nil, nil, func(candidate *domain.CoordinationCase) error {
		return candidate.ReplaceProposal(domain.TransmitterProposal{
			FrequencyHz: 100_000_000, BandwidthHz: 200_000, EIRPDbm: 5,
			AntennaGainDbi: 2, AntennaHeightM: 30, Latitude: 23.1, Longitude: 113.3,
			EmissionClass: "F3E", Rationale: "依据",
		}, createdAt.Add(time.Minute))
	})
	if err == nil {
		t.Fatal("账本追加失败时应返回错误")
	}

	observed, err := repository.Get(value.ID)
	if err != nil {
		t.Fatal(err)
	}
	if observed.Version != value.Version || observed.CurrentProposalRevision != value.CurrentProposalRevision {
		t.Fatalf("账本写入失败后仍发布了内存状态: version=%d proposalRevision=%d", observed.Version, observed.CurrentProposalRevision)
	}
}
