package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/domain"
)

func TestRepositoryRecoveryAndIdempotency(t *testing.T) {
	directory := t.TempDir()
	repository, err := Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 25, 2, 0, 0, 0, time.UTC)
	value, _ := domain.NewCase("case-store", "持久化测试", "CN-44", "申请人", now)
	value, err = repository.Create(value, "case_created", "工程师", nil)
	if err != nil {
		t.Fatal(err)
	}
	value, _, err = repository.Update(value.ID, value.Version, "proposal_revised", "工程师", nil, nil, func(candidate *domain.CoordinationCase) error {
		return candidate.ReplaceProposal(domain.TransmitterProposal{FrequencyHz: 100_000_000, BandwidthHz: 200_000, EIRPDbm: 5, AntennaGainDbi: 2, AntennaHeightM: 30, Latitude: 23.1, Longitude: 113.3, EmissionClass: "F3E", Rationale: "依据"}, now.Add(time.Minute))
	})
	if err != nil {
		t.Fatal(err)
	}
	value, _, err = repository.Update(value.ID, value.Version, "receiver_added", "工程师", nil, nil, func(candidate *domain.CoordinationCase) error {
		return candidate.AddReceiver(domain.ProtectedReceiver{ID: "rx-1", Label: "保护点", Latitude: 23.2, Longitude: 113.4, ReceiveFrequencyHz: 100_000_000, ProtectionThresholdDbm: -60, AntennaGainDbi: 1, TerrainClass: "suburban", EvidenceRef: "E-1"}, now.Add(2*time.Minute))
	})
	if err != nil {
		t.Fatal(err)
	}
	fingerprint, _ := Fingerprint(map[string]any{"caseId": value.ID, "expectedVersion": value.Version})
	idem := &IdempotencyRequest{Key: "submit-key", Fingerprint: fingerprint}
	originalVersion := value.Version
	value, replay, err := repository.Update(value.ID, originalVersion, "analysis_submitted", "工程师", nil, idem, func(candidate *domain.CoordinationCase) error {
		return candidate.SubmitForAnalysis(now.Add(3 * time.Minute))
	})
	if err != nil || replay {
		t.Fatalf("first submit = replay %v, error %v", replay, err)
	}
	reopened, err := Open(directory)
	if err != nil {
		t.Fatalf("Open(recovery) error = %v", err)
	}
	recovered, err := reopened.Get(value.ID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Version != value.Version || recovered.State != domain.StateAnalysisPending {
		t.Fatalf("recovered case = %+v", recovered)
	}
	replayed, replay, err := reopened.Update(value.ID, originalVersion, "analysis_submitted", "工程师", nil, idem, func(candidate *domain.CoordinationCase) error {
		t.Fatal("idempotent mutation was executed")
		return nil
	})
	if err != nil || !replay || replayed.Version != value.Version {
		t.Fatalf("replay result = %+v, %v, %v", replayed, replay, err)
	}
	conflict := &IdempotencyRequest{Key: idem.Key, Fingerprint: "different"}
	_, _, err = reopened.Update(value.ID, originalVersion, "analysis_submitted", "工程师", nil, conflict, func(candidate *domain.CoordinationCase) error { return nil })
	if err == nil || !domain.IsCode(err, domain.CodeIdempotency) {
		t.Fatalf("idempotency conflict error = %v", err)
	}
	audit, err := reopened.Audit(value.ID)
	if err != nil || len(audit) != 4 {
		t.Fatalf("audit count = %d, error = %v", len(audit), err)
	}
}

func TestOpenRejectsTamperedLedger(t *testing.T) {
	directory := t.TempDir()
	repository, err := Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	value, _ := domain.NewCase("case-tamper", "篡改测试", "CN-51", "申请人", time.Now().UTC())
	if _, err := repository.Create(value, "case_created", "原操作人", nil); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "events.jsonl")
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var event ledgerEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		t.Fatal(err)
	}
	event.Actor = "被篡改的操作人"
	tampered, _ := json.Marshal(event)
	tampered = append(tampered, '\n')
	if err := os.WriteFile(path, tampered, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(directory); err == nil || !domain.IsCode(err, domain.CodeIntegrity) {
		t.Fatalf("Open(tampered) error = %v", err)
	}
}
