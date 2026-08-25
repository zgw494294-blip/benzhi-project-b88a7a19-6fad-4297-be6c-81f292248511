package analysis

import (
	"testing"
	"time"

	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/domain"
)

func TestEvaluateAndVerifyAssessment(t *testing.T) {
	engine := NewEngine()
	now := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	proposal := domain.TransmitterProposal{
		CaseID: "case-1", Revision: 1, FrequencyHz: 100_000_000, BandwidthHz: 200_000,
		EIRPDbm: 10, AntennaGainDbi: 3, AntennaHeightM: 40, Latitude: 31.2,
		Longitude: 121.4, EmissionClass: "F3E", Rationale: "现场勘测", SubmittedAt: now,
	}
	receivers := []domain.ProtectedReceiver{
		{ID: "rx-b", CaseID: "case-1", Label: "远端", Latitude: 31.5, Longitude: 121.7, ReceiveFrequencyHz: 101_000_000, ProtectionThresholdDbm: -65, AntennaGainDbi: 2, TerrainClass: "mountain", EvidenceRef: "E-2"},
		{ID: "rx-a", CaseID: "case-1", Label: "近端", Latitude: 31.3, Longitude: 121.5, ReceiveFrequencyHz: 100_000_000, ProtectionThresholdDbm: -60, AntennaGainDbi: 1, TerrainClass: "urban", EvidenceRef: "E-1"},
	}
	assessment, err := engine.Evaluate(Input{CaseID: "case-1", AssessmentID: "assessment-1", Revision: 1, Proposal: proposal, Receivers: receivers, CreatedAt: now.Add(time.Minute)})
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if len(assessment.PointResults) != 2 {
		t.Fatalf("point result count = %d", len(assessment.PointResults))
	}
	if assessment.PointResults[0].ReceiverID != "rx-a" {
		t.Fatalf("results were not normalized by receiver ID")
	}
	if assessment.InputDigest == "" || assessment.AlgorithmVersion != AlgorithmVersion {
		t.Fatalf("assessment identity fields are incomplete")
	}
	for _, point := range assessment.PointResults {
		if point.DistanceKM <= 0 || point.FreeSpacePathLossDB <= 0 {
			t.Fatalf("invalid calculation for %s: %+v", point.ReceiverID, point)
		}
		if len(point.Rules) != 4 {
			t.Fatalf("rule count for %s = %d", point.ReceiverID, len(point.Rules))
		}
	}
	report, err := engine.VerifyAssessment(assessment, proposal, receivers)
	if err != nil {
		t.Fatalf("VerifyAssessment() error = %v", err)
	}
	if !report.Valid {
		t.Fatalf("verification report = %+v", report)
	}
	assessment.PointResults[0].MarginDB++
	report, err = engine.VerifyAssessment(assessment, proposal, receivers)
	if err != nil {
		t.Fatalf("VerifyAssessment(tampered) error = %v", err)
	}
	if report.Valid || report.PointResultsValid {
		t.Fatalf("tampered result was accepted: %+v", report)
	}
}

func TestEvaluateRejectsInvalidInput(t *testing.T) {
	engine := NewEngine()
	_, err := engine.Evaluate(Input{Proposal: domain.TransmitterProposal{FrequencyHz: 1}, Receivers: nil})
	if err == nil || !domain.IsCode(err, domain.CodeValidation) {
		t.Fatalf("error = %v, want validation_error", err)
	}
}
