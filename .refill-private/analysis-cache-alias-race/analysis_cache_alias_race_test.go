package analysis_cache_alias_race_test

import (
	"sync"
	"testing"
	"time"

	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/analysis"
	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/domain"
)

func TestConcurrentEvaluateDoesNotShareCachedResultMemory(t *testing.T) {
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	input := analysis.Input{
		CaseID: "case-cache-race", AssessmentID: "assessment-cache-race", Revision: 1, CreatedAt: now.Add(time.Minute),
		Proposal: domain.TransmitterProposal{
			CaseID: "case-cache-race", Revision: 1, FrequencyHz: 100_100_000, BandwidthHz: 200_000,
			EIRPDbm: 8, AntennaGainDbi: 3, AntennaHeightM: 50, Latitude: 31.2304,
			Longitude: 121.4737, EmissionClass: "F3E", Rationale: "缓存并发复现", SubmittedAt: now,
		},
		Receivers: []domain.ProtectedReceiver{{
			ID: "rx-cache-race", CaseID: "case-cache-race", Label: "缓存保护点",
			Latitude: 31.3204, Longitude: 121.5237, ReceiveFrequencyHz: 100_100_000,
			ProtectionThresholdDbm: -65, AntennaGainDbi: 2, TerrainClass: "urban", EvidenceRef: "CACHE-RACE-001",
		}},
	}

	engine := analysis.NewEngine()
	first, err := engine.Evaluate(input)
	if err != nil {
		t.Fatalf("首次 Evaluate() error = %v", err)
	}
	originalRule := first.PointResults[0].Rules[0]
	if _, err := engine.Evaluate(input); err != nil {
		t.Fatalf("缓存预热 Evaluate() error = %v", err)
	}

	start := make(chan struct{})
	ready := make(chan struct{}, 2)
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		ready <- struct{}{}
		<-start
		for i := 0; i < 2000; i++ {
			first.PointResults[0].Rules[0] = "调用方改写的规则"
		}
	}()
	go func() {
		defer workers.Done()
		ready <- struct{}{}
		<-start
		for i := 0; i < 2000; i++ {
			cached, evaluateErr := engine.Evaluate(input)
			if evaluateErr != nil {
				t.Errorf("缓存 Evaluate() error = %v", evaluateErr)
				return
			}
			_ = cached.PointResults[0].Rules[0]
		}
	}()
	<-ready
	<-ready
	close(start)
	workers.Wait()
	after, err := engine.Evaluate(input)
	if err != nil {
		t.Fatalf("并发后 Evaluate() error = %v", err)
	}
	if after.PointResults[0].Rules[0] != originalRule {
		t.Fatalf("缓存结果被调用方污染: got %q, want %q", after.PointResults[0].Rules[0], originalRule)
	}
}
