package analysis

import (
	"fmt"
	"math"

	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/domain"
)

type VerificationReport struct {
	Valid              bool     `json:"valid"`
	InputDigestValid   bool     `json:"inputDigestValid"`
	AlgorithmSupported bool     `json:"algorithmSupported"`
	OutcomeValid       bool     `json:"outcomeValid"`
	PointResultsValid  bool     `json:"pointResultsValid"`
	ComputedDigest     string   `json:"computedDigest"`
	Issues             []string `json:"issues"`
}

func (e *Engine) VerifyAssessment(assessment domain.InterferenceAssessment, proposal domain.TransmitterProposal, receivers []domain.ProtectedReceiver) (VerificationReport, error) {
	report := VerificationReport{AlgorithmSupported: assessment.AlgorithmVersion == e.Version, Issues: []string{}}
	inputValid, computedDigest, err := e.VerifyInput(assessment, proposal, receivers)
	if err != nil {
		return VerificationReport{}, err
	}
	report.InputDigestValid = inputValid
	report.ComputedDigest = computedDigest
	if !report.AlgorithmSupported {
		report.Issues = append(report.Issues, "分析修订的算法版本不是当前可复算版本")
	}
	if !report.InputDigestValid {
		report.Issues = append(report.Issues, "规范化输入摘要与分析修订记录不一致")
	}
	if !report.AlgorithmSupported {
		return report, nil
	}
	recomputed, err := e.evaluate(Input{
		CaseID: assessment.CaseID, AssessmentID: assessment.ID, Revision: assessment.Revision,
		Proposal: proposal, Receivers: receivers, CreatedAt: assessment.CreatedAt,
	}, false)
	if err != nil {
		return VerificationReport{}, err
	}
	report.OutcomeValid = assessment.OverallOutcome == recomputed.OverallOutcome && closeEnough(assessment.MinimumMarginDB, recomputed.MinimumMarginDB)
	if !report.OutcomeValid {
		report.Issues = append(report.Issues, "总体结论或最小保护裕量与复算结果不一致")
	}
	report.PointResultsValid = comparePointResults(assessment.PointResults, recomputed.PointResults, &report.Issues)
	report.Valid = report.AlgorithmSupported && report.InputDigestValid && report.OutcomeValid && report.PointResultsValid
	return report, nil
}

func comparePointResults(stored, computed []domain.PointAssessment, issues *[]string) bool {
	if len(stored) != len(computed) {
		*issues = append(*issues, fmt.Sprintf("逐点结果数量不一致：记录 %d，复算 %d", len(stored), len(computed)))
		return false
	}
	valid := true
	for index := range stored {
		left, right := stored[index], computed[index]
		if left.ReceiverID != right.ReceiverID || left.ReceiverLabel != right.ReceiverLabel {
			*issues = append(*issues, fmt.Sprintf("第 %d 个逐点结果绑定的接收点不一致", index+1))
			valid = false
			continue
		}
		if !pointNumbersEqual(left, right) {
			*issues = append(*issues, fmt.Sprintf("接收点 %s 的计算中间量与复算结果不一致", left.ReceiverID))
			valid = false
		}
		if left.Passed != right.Passed {
			*issues = append(*issues, fmt.Sprintf("接收点 %s 的通过结论与复算结果不一致", left.ReceiverID))
			valid = false
		}
		if len(left.Rules) != len(right.Rules) {
			*issues = append(*issues, fmt.Sprintf("接收点 %s 的规则依据数量不一致", left.ReceiverID))
			valid = false
		} else {
			for ruleIndex := range left.Rules {
				if left.Rules[ruleIndex] != right.Rules[ruleIndex] {
					*issues = append(*issues, fmt.Sprintf("接收点 %s 的第 %d 条规则依据不一致", left.ReceiverID, ruleIndex+1))
					valid = false
				}
			}
		}
	}
	return valid
}

func pointNumbersEqual(left, right domain.PointAssessment) bool {
	return closeEnough(left.DistanceKM, right.DistanceKM) &&
		closeEnough(left.FrequencySeparationHz, right.FrequencySeparationHz) &&
		closeEnough(left.FreeSpacePathLossDB, right.FreeSpacePathLossDB) &&
		closeEnough(left.HeightCorrectionDB, right.HeightCorrectionDB) &&
		closeEnough(left.TerrainCorrectionDB, right.TerrainCorrectionDB) &&
		closeEnough(left.FrequencyRejectionDB, right.FrequencyRejectionDB) &&
		closeEnough(left.ReceivedInterferenceDBm, right.ReceivedInterferenceDBm) &&
		closeEnough(left.ThresholdDBm, right.ThresholdDBm) &&
		closeEnough(left.MarginDB, right.MarginDB)
}

func closeEnough(left, right float64) bool {
	return math.Abs(left-right) <= 0.000001
}
