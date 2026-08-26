package projection_failure_state

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/domain"
	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/store"
)

func TestProjectionFailureDoesNotPublishState(t *testing.T) {
	directory := t.TempDir()
	repository, err := store.Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 25, 3, 0, 0, 0, time.UTC)
	value, err := domain.NewCase("case-projection-failure", "投影故障测试", "CN-44", "申请人", now)
	if err != nil {
		t.Fatal(err)
	}
	value, err = repository.Create(value, "case_created", "工程师", nil)
	if err != nil {
		t.Fatal(err)
	}

	snapshotPath := filepath.Join(directory, "projection.json")
	if err := os.Remove(snapshotPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(snapshotPath, 0o700); err != nil {
		t.Fatal(err)
	}
	proposal := domain.TransmitterProposal{
		FrequencyHz: 100_000_000, BandwidthHz: 200_000, EIRPDbm: 5,
		AntennaGainDbi: 2, AntennaHeightM: 30, Latitude: 23.1, Longitude: 113.3,
		EmissionClass: "F3E", Rationale: "依据",
	}
	if _, _, err := repository.Update(value.ID, value.Version, "proposal_revised", "工程师", nil, nil, func(candidate *domain.CoordinationCase) error {
		return candidate.ReplaceProposal(proposal, now.Add(time.Minute))
	}); err == nil || !domain.IsCode(err, domain.CodeIntegrity) {
		t.Fatalf("projection failure error = %v", err)
	}

	current, err := repository.Get(value.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.Version != value.Version || current.CurrentProposalRevision != value.CurrentProposalRevision {
		t.Errorf("failed update published in-memory state: version=%d proposalRevision=%d", current.Version, current.CurrentProposalRevision)
	}

	if err := os.Remove(snapshotPath); err != nil {
		t.Fatal(err)
	}
	reopened, err := store.Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := reopened.Get(value.ID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Version != value.Version || recovered.CurrentProposalRevision != value.CurrentProposalRevision {
		t.Errorf("failed update persisted in ledger: version=%d proposalRevision=%d", recovered.Version, recovered.CurrentProposalRevision)
	}
}
