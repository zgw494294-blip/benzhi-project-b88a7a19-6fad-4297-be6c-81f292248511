package contextcancelanalysis

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/analysis"
	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/coordination"
	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/domain"
	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/store"
)

func TestContextCancellationDoesNotCommitAnalysis(t *testing.T) {
	t.Run("waiting for transaction", func(t *testing.T) {
		started := make(chan struct{})
		release := make(chan struct{})
		repository := newBlockingRepository(t, started, release)
		service := coordination.NewServiceWithDependencies(repository, successfulEngine{}, fixedClock, fixedID)
		ctx, cancel := context.WithCancel(context.Background())
		result := make(chan error, 1)

		go func() {
			_, err := service.RunAnalysisContext(ctx, repository.value.ID, analysisCommand(repository.value.Version))
			result <- err
		}()
		<-started
		cancel()
		close(release)

		if err := <-result; !errors.Is(err, context.Canceled) {
			t.Fatalf("RunAnalysisContext error = %v, want context.Canceled", err)
		}
		if repository.committed.Load() {
			t.Fatal("canceled analysis was committed after waiting for the transaction")
		}
	})

	t.Run("during engine evaluation", func(t *testing.T) {
		started := make(chan struct{})
		release := make(chan struct{})
		repository := newBlockingRepository(t, nil, nil)
		engine := &blockingEngine{started: started, release: release}
		service := coordination.NewServiceWithDependencies(repository, engine, fixedClock, fixedID)
		ctx, cancel := context.WithCancel(context.Background())
		result := make(chan error, 1)

		go func() {
			_, err := service.RunAnalysisContext(ctx, repository.value.ID, analysisCommand(repository.value.Version))
			result <- err
		}()
		<-started
		cancel()
		close(release)

		if err := <-result; !errors.Is(err, context.Canceled) {
			t.Fatalf("RunAnalysisContext error = %v, want context.Canceled", err)
		}
		if repository.committed.Load() {
			t.Fatal("canceled analysis was committed after engine evaluation")
		}
	})
}

type blockingRepository struct {
	value          *domain.CoordinationCase
	updateStarted  chan struct{}
	continueUpdate chan struct{}
	committed      atomic.Bool
}

func newBlockingRepository(t *testing.T, started, release chan struct{}) *blockingRepository {
	t.Helper()
	now := fixedClock()
	value, err := domain.NewCase("case-context", "取消传播复现", "CN-44", "测试申请人", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := value.ReplaceProposal(domain.TransmitterProposal{
		FrequencyHz: 100_000_000, BandwidthHz: 200_000, EIRPDbm: 5,
		AntennaGainDbi: 2, AntennaHeightM: 30, Latitude: 23.1, Longitude: 113.3,
		EmissionClass: "F3E", Rationale: "测试依据",
	}, now); err != nil {
		t.Fatal(err)
	}
	if err := value.AddReceiver(domain.ProtectedReceiver{
		ID: "rx-1", Label: "保护点", Latitude: 23.2, Longitude: 113.4,
		ReceiveFrequencyHz: 100_000_000, ProtectionThresholdDbm: -60,
		AntennaGainDbi: 1, TerrainClass: "suburban", EvidenceRef: "E-1",
	}, now); err != nil {
		t.Fatal(err)
	}
	if err := value.SubmitForAnalysis(now); err != nil {
		t.Fatal(err)
	}
	return &blockingRepository{value: value, updateStarted: started, continueUpdate: release}
}

func (r *blockingRepository) Create(value *domain.CoordinationCase, action, actor string, details map[string]any) (*domain.CoordinationCase, error) {
	return nil, errors.New("unexpected Create call")
}

func (r *blockingRepository) Update(caseID string, expectedVersion int64, action, actor string, details map[string]any, idem *store.IdempotencyRequest, mutate store.Mutation) (*domain.CoordinationCase, bool, error) {
	if r.updateStarted != nil {
		close(r.updateStarted)
		<-r.continueUpdate
	}
	next := cloneCase(r.value)
	if err := mutate(next); err != nil {
		return nil, false, err
	}
	r.committed.Store(true)
	r.value = next
	return cloneCase(next), false, nil
}

func (r *blockingRepository) Get(caseID string) (*domain.CoordinationCase, error) {
	return cloneCase(r.value), nil
}

func (r *blockingRepository) Audit(caseID string) ([]domain.AuditEntry, error) {
	return nil, nil
}

type successfulEngine struct{}

func (successfulEngine) Evaluate(input analysis.Input) (domain.InterferenceAssessment, error) {
	return completedAssessment(input), nil
}

func (successfulEngine) VerifyInput(domain.InterferenceAssessment, domain.TransmitterProposal, []domain.ProtectedReceiver) (bool, string, error) {
	return true, "digest", nil
}

func (successfulEngine) VerifyAssessment(domain.InterferenceAssessment, domain.TransmitterProposal, []domain.ProtectedReceiver) (analysis.VerificationReport, error) {
	return analysis.VerificationReport{Valid: true}, nil
}

type blockingEngine struct {
	started chan struct{}
	release chan struct{}
}

func (e *blockingEngine) Evaluate(input analysis.Input) (domain.InterferenceAssessment, error) {
	close(e.started)
	<-e.release
	return completedAssessment(input), nil
}

func (e *blockingEngine) VerifyInput(domain.InterferenceAssessment, domain.TransmitterProposal, []domain.ProtectedReceiver) (bool, string, error) {
	return true, "digest", nil
}

func (e *blockingEngine) VerifyAssessment(domain.InterferenceAssessment, domain.TransmitterProposal, []domain.ProtectedReceiver) (analysis.VerificationReport, error) {
	return analysis.VerificationReport{Valid: true}, nil
}

func completedAssessment(input analysis.Input) domain.InterferenceAssessment {
	return domain.InterferenceAssessment{
		ID: input.AssessmentID, CaseID: input.CaseID, Revision: input.Revision,
		ProposalRevision: input.Proposal.Revision, AlgorithmVersion: analysis.AlgorithmVersion,
		InputDigest: "digest", OverallOutcome: "pass", MinimumMarginDB: 10,
		PointResults: []domain.PointAssessment{{ReceiverID: input.Receivers[0].ID, MarginDB: 10, Passed: true}},
		CreatedAt:    input.CreatedAt,
	}
}

func cloneCase(value *domain.CoordinationCase) *domain.CoordinationCase {
	result := *value
	result.Proposals = append([]domain.TransmitterProposal(nil), value.Proposals...)
	result.Receivers = append([]domain.ProtectedReceiver(nil), value.Receivers...)
	result.Assessments = append([]domain.InterferenceAssessment(nil), value.Assessments...)
	return &result
}

func analysisCommand(version int64) coordination.VersionedCommand {
	return coordination.VersionedCommand{ExpectedVersion: version, Actor: "分析员"}
}

func fixedClock() time.Time {
	return time.Date(2026, 8, 25, 4, 0, 0, 0, time.UTC)
}

func fixedID(prefix string) string {
	return prefix + "-context"
}
