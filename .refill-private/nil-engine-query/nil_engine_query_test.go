package nilenginequery

import (
	"testing"
	"time"

	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/analysis"
	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/coordination"
	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/domain"
	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/store"
)

type repositoryStub struct {
	value *domain.CoordinationCase
}

func (r repositoryStub) Create(*domain.CoordinationCase, string, string, map[string]any) (*domain.CoordinationCase, error) {
	return nil, nil
}

func (r repositoryStub) Update(string, int64, string, string, map[string]any, *store.IdempotencyRequest, store.Mutation) (*domain.CoordinationCase, bool, error) {
	return nil, false, nil
}

func (r repositoryStub) Get(string) (*domain.CoordinationCase, error) { return r.value, nil }
func (r repositoryStub) Audit(string) ([]domain.AuditEntry, error)    { return nil, nil }

func TestNilEngineAnalysisQueryReturnsErrorInsteadOfPanicking(t *testing.T) {
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	value, err := domain.NewCase("case-nil-engine", "查询测试", "CN-11", "申请人", now)
	if err != nil {
		t.Fatal(err)
	}
	proposal := domain.TransmitterProposal{CaseID: value.ID, Revision: 1, FrequencyHz: 100_000_000, BandwidthHz: 200_000, EIRPDbm: 0, AntennaGainDbi: 1, AntennaHeightM: 20, Latitude: 31, Longitude: 121, EmissionClass: "F3E", Rationale: "依据", SubmittedAt: now}
	if err := value.ReplaceProposal(proposal, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	receiver := domain.ProtectedReceiver{ID: "rx-1", CaseID: value.ID, Label: "保护点", Latitude: 31.1, Longitude: 121.1, ReceiveFrequencyHz: 100_000_000, ProtectionThresholdDbm: -50, AntennaGainDbi: 0, TerrainClass: "urban", EvidenceRef: "E-1"}
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
	service := coordination.NewServiceWithDependencies(repositoryStub{value: value}, nil, func() time.Time { return now }, func(string) string { return "id" })
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("nil 分析引擎导致查询崩溃: %v", recovered)
		}
	}()
	if _, err := service.GetAnalysisBasis(value.ID); err == nil {
		t.Fatal("空分析引擎应返回完整性错误")
	}
}
