package domain_test

import (
	"testing"
	"time"

	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/analysis"
	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/domain"
)

func TestAggregateRejectsStaleBindingsAndFreezesApprovedRevision(t *testing.T) {
	now := time.Date(2026, 8, 25, 1, 0, 0, 0, time.UTC)
	value, err := domain.NewCase("case-domain", "试播协调案", "CN-11", "测试申请人", now)
	if err != nil {
		t.Fatal(err)
	}
	proposal := domain.TransmitterProposal{FrequencyHz: 99_900_000, BandwidthHz: 200_000, EIRPDbm: 5, AntennaGainDbi: 2, AntennaHeightM: 30, Latitude: 39.9, Longitude: 116.4, EmissionClass: "F3E", Rationale: "测试依据"}
	if err := value.ReplaceProposal(proposal, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	receiver := domain.ProtectedReceiver{ID: "rx-1", Label: "保护点", Latitude: 39.95, Longitude: 116.45, ReceiveFrequencyHz: 99_900_000, ProtectionThresholdDbm: -55, AntennaGainDbi: 1, TerrainClass: "urban", EvidenceRef: "EVIDENCE-1"}
	if err := value.AddReceiver(receiver, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := value.SubmitForAnalysis(now.Add(3 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	latest, _ := value.LatestProposal()
	assessment, err := analysis.NewEngine().Evaluate(analysis.Input{CaseID: value.ID, AssessmentID: "assessment-1", Revision: 1, Proposal: latest, Receivers: value.Receivers, CreatedAt: now.Add(4 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	if err := value.AttachAssessment(assessment, now.Add(4*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := value.SubmitForReview(now.Add(5 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	stale := domain.ReviewDecision{ID: "review-stale", AssessmentRevision: 0, Reviewer: "复核员", Decision: "approved", Reason: "错误绑定"}
	if err := value.DecideReview(stale, now.Add(6*time.Minute)); err == nil || !domain.IsCode(err, domain.CodeStateConflict) {
		t.Fatalf("stale review error = %v", err)
	}
	approved := domain.ReviewDecision{ID: "review-1", AssessmentRevision: 1, Reviewer: "复核员", Findings: []domain.ReviewFinding{{Item: "裕量", Severity: "info", Comment: "满足要求"}}, Decision: "approved", Reason: "复算通过"}
	if err := value.DecideReview(approved, now.Add(6*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := value.Freeze("负责人", now.Add(7*time.Minute)); err != nil {
		t.Fatal(err)
	}
	valid, digest, err := domain.VerifyFrozen(value)
	if err != nil || !valid || digest == "" {
		t.Fatalf("VerifyFrozen() = %v, %q, %v", valid, digest, err)
	}
	if err := value.ReplaceProposal(proposal, now.Add(8*time.Minute)); err == nil || !domain.IsCode(err, domain.CodeStateConflict) {
		t.Fatalf("frozen mutation error = %v", err)
	}
	authorization := domain.TrialAuthorization{AuthorizationNo: "TA-1", ValidFrom: now.Add(24 * time.Hour), ValidUntil: now.Add(48 * time.Hour), Conditions: []string{"按冻结参数运行"}, Issuer: "授权人"}
	if err := value.Authorize(authorization, now.Add(8*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if value.State != domain.StateAuthorized {
		t.Fatalf("state = %s", value.State)
	}
	if err := domain.ValidateCaseIntegrity(value); err != nil {
		t.Fatalf("ValidateCaseIntegrity() error = %v", err)
	}
}

func TestBlockingFindingCannotApprove(t *testing.T) {
	value := buildUnderReviewCase(t)
	review := domain.ReviewDecision{ID: "review-block", AssessmentRevision: 1, Reviewer: "复核员", Findings: []domain.ReviewFinding{{Item: "功率", Severity: "blocking", Comment: "超过限制"}}, Decision: "approved", Reason: "不应批准"}
	err := value.DecideReview(review, time.Now().UTC())
	if err == nil || !domain.IsCode(err, domain.CodeStateConflict) {
		t.Fatalf("error = %v", err)
	}
}

func buildUnderReviewCase(t *testing.T) *domain.CoordinationCase {
	t.Helper()
	now := time.Now().UTC().Add(-time.Hour)
	value, _ := domain.NewCase("case-review", "复核案", "CN-31", "申请人", now)
	proposal := domain.TransmitterProposal{FrequencyHz: 100_000_000, BandwidthHz: 200_000, EIRPDbm: 0, AntennaGainDbi: 1, AntennaHeightM: 20, Latitude: 31, Longitude: 121, EmissionClass: "F3E", Rationale: "依据"}
	_ = value.ReplaceProposal(proposal, now.Add(time.Minute))
	_ = value.AddReceiver(domain.ProtectedReceiver{ID: "rx", Label: "点", Latitude: 31.1, Longitude: 121.1, ReceiveFrequencyHz: 100_000_000, ProtectionThresholdDbm: -50, AntennaGainDbi: 0, TerrainClass: "urban", EvidenceRef: "E"}, now.Add(2*time.Minute))
	_ = value.SubmitForAnalysis(now.Add(3 * time.Minute))
	latest, _ := value.LatestProposal()
	assessment, _ := analysis.NewEngine().Evaluate(analysis.Input{CaseID: value.ID, AssessmentID: "a", Revision: 1, Proposal: latest, Receivers: value.Receivers, CreatedAt: now.Add(4 * time.Minute)})
	_ = value.AttachAssessment(assessment, now.Add(4*time.Minute))
	_ = value.SubmitForReview(now.Add(5 * time.Minute))
	return value
}
