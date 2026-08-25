package analysis

import (
	"math"
	"sort"

	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/domain"
)

func (e *Engine) Evaluate(input Input) (domain.InterferenceAssessment, error) {
	return e.evaluate(input, true)
}

func (e *Engine) evaluate(input Input, useCache bool) (domain.InterferenceAssessment, error) {
	if e == nil || e.Version == "" {
		return domain.InterferenceAssessment{}, domain.NewError(domain.CodeIntegrity, "分析引擎版本缺失")
	}
	if err := domain.ValidateProposal(input.Proposal); err != nil {
		return domain.InterferenceAssessment{}, err
	}
	if len(input.Receivers) == 0 {
		return domain.InterferenceAssessment{}, domain.ValidationError(domain.Violation("receivers", "min_items", "至少需要一个受保护点"))
	}
	receivers := append([]domain.ProtectedReceiver(nil), input.Receivers...)
	sort.Slice(receivers, func(i, j int) bool { return receivers[i].ID < receivers[j].ID })
	for _, receiver := range receivers {
		if err := domain.ValidateReceiver(receiver); err != nil {
			return domain.InterferenceAssessment{}, err
		}
	}
	digest, err := inputDigest(e.Version, input.CaseID, input.Proposal, receivers)
	if err != nil {
		return domain.InterferenceAssessment{}, err
	}
	if useCache {
		if cached, ok := e.cachedResult(digest); ok {
			return assessmentFromResult(input, e.Version, digest, cached), nil
		}
	}
	results := make([]domain.PointAssessment, 0, len(receivers))
	minimumMargin := math.Inf(1)
	overallPass := true
	for _, receiver := range receivers {
		result := e.evaluatePoint(input.Proposal, receiver)
		results = append(results, result)
		if result.MarginDB < minimumMargin {
			minimumMargin = result.MarginDB
		}
		if !result.Passed {
			overallPass = false
		}
	}
	outcome := "pass"
	if !overallPass {
		outcome = "fail"
	}
	result := evaluationResult{PointResults: results, OverallOutcome: outcome, MinimumMarginDB: round(minimumMargin)}
	if useCache {
		e.cacheResult(digest, result)
	}
	return assessmentFromResult(input, e.Version, digest, result), nil
}

func assessmentFromResult(input Input, version, digest string, result evaluationResult) domain.InterferenceAssessment {
	return domain.InterferenceAssessment{
		ID: input.AssessmentID, CaseID: input.CaseID, Revision: input.Revision,
		ProposalRevision: input.Proposal.Revision, AlgorithmVersion: version,
		InputDigest: digest, PointResults: result.PointResults, OverallOutcome: result.OverallOutcome,
		MinimumMarginDB: result.MinimumMarginDB, CreatedAt: input.CreatedAt.UTC(),
	}
}

func (e *Engine) cachedResult(digest string) (evaluationResult, bool) {
	e.cacheMu.RLock()
	defer e.cacheMu.RUnlock()
	result, ok := e.cache[digest]
	return result, ok
}

func (e *Engine) cacheResult(digest string, result evaluationResult) {
	e.cacheMu.Lock()
	defer e.cacheMu.Unlock()
	if e.cache == nil {
		e.cache = make(map[string]evaluationResult)
	}
	e.cache[digest] = result
}

func (e *Engine) evaluatePoint(proposal domain.TransmitterProposal, receiver domain.ProtectedReceiver) domain.PointAssessment {
	distance := greatCircleDistanceKM(proposal.Latitude, proposal.Longitude, receiver.Latitude, receiver.Longitude)
	separation := math.Abs(proposal.FrequencyHz - receiver.ReceiveFrequencyHz)
	pathLoss := freeSpacePathLossDB(distance, proposal.FrequencyHz)
	heightCorrection := heightCorrectionDB(proposal.AntennaHeightM)
	terrainCorrection, terrainRule := terrainCorrectionDB(receiver.TerrainClass)
	frequencyRejection, frequencyRule := frequencyRejectionDB(separation, proposal.BandwidthHz)
	received := proposal.EIRPDbm + receiver.AntennaGainDbi - pathLoss + heightCorrection - terrainCorrection - frequencyRejection
	margin := receiver.ProtectionThresholdDbm - received
	passed := margin >= 0
	thresholdRule := "计算干扰电平不高于保护门限"
	if !passed {
		thresholdRule = "计算干扰电平高于保护门限"
	}
	return domain.PointAssessment{
		ReceiverID: receiver.ID, ReceiverLabel: receiver.Label, DistanceKM: round(distance),
		FrequencySeparationHz: round(separation), FreeSpacePathLossDB: round(pathLoss),
		HeightCorrectionDB: round(heightCorrection), TerrainCorrectionDB: round(terrainCorrection),
		FrequencyRejectionDB: round(frequencyRejection), ReceivedInterferenceDBm: round(received),
		ThresholdDBm: round(receiver.ProtectionThresholdDbm), MarginDB: round(margin), Passed: passed,
		Rules: []string{"使用 ITU 自由空间路径损耗形式计算基础损耗", terrainRule, frequencyRule, thresholdRule},
	}
}

func (e *Engine) VerifyInput(assessment domain.InterferenceAssessment, proposal domain.TransmitterProposal, receivers []domain.ProtectedReceiver) (bool, string, error) {
	digest, err := inputDigest(e.Version, assessment.CaseID, proposal, receivers)
	if err != nil {
		return false, "", err
	}
	return digest == assessment.InputDigest, digest, nil
}
