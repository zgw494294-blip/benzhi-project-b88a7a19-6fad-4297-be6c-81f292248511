package domain

import "fmt"

var validStates = map[CaseState]bool{
	StateDraft: true, StateAnalysisPending: true, StateAnalyzed: true,
	StateUnderReview: true, StateRevisionRequired: true, StateApproved: true,
	StateFrozen: true, StateAuthorized: true,
}

func ValidateCaseIntegrity(value *CoordinationCase) error {
	if value == nil {
		return NewError(CodeIntegrity, "协调案聚合不能为空")
	}
	if value.ID == "" {
		return NewError(CodeIntegrity, "协调案 ID 缺失")
	}
	if !validStates[value.State] {
		return NewError(CodeIntegrity, "协调案状态无效")
	}
	if value.Version < 1 {
		return NewError(CodeIntegrity, "协调案版本必须为正整数")
	}
	if value.CreatedAt.IsZero() || value.UpdatedAt.IsZero() || value.UpdatedAt.Before(value.CreatedAt) {
		return NewError(CodeIntegrity, "协调案时间边界无效")
	}
	if err := ValidateNewCase(value.Title, value.RegionCode, value.Applicant); err != nil {
		return integrityFromValidation("协调案基础字段无效", err)
	}
	if err := validateProposalHistory(value); err != nil {
		return err
	}
	if err := validateReceivers(value); err != nil {
		return err
	}
	if err := validateAssessmentHistory(value); err != nil {
		return err
	}
	if err := validateReviewHistory(value); err != nil {
		return err
	}
	if err := validateReviewResponses(value); err != nil {
		return err
	}
	if err := validateStateProjection(value); err != nil {
		return err
	}
	return nil
}

func validateProposalHistory(value *CoordinationCase) error {
	if value.CurrentProposalRevision < 0 {
		return NewError(CodeIntegrity, "当前候选修订号不能为负数")
	}
	if len(value.Proposals) == 0 {
		if value.CurrentProposalRevision != 0 {
			return NewError(CodeIntegrity, "候选修订索引指向不存在的记录")
		}
		return nil
	}
	if value.CurrentProposalRevision != len(value.Proposals) {
		return NewError(CodeIntegrity, "当前候选修订号与历史长度不一致")
	}
	previousTime := value.CreatedAt
	for index, proposal := range value.Proposals {
		expectedRevision := index + 1
		if proposal.CaseID != value.ID || proposal.Revision != expectedRevision {
			return NewError(CodeIntegrity, fmt.Sprintf("候选修订 %d 的归属或序号无效", expectedRevision))
		}
		if proposal.SubmittedAt.Before(previousTime) {
			return NewError(CodeIntegrity, fmt.Sprintf("候选修订 %d 的时间早于前序记录", expectedRevision))
		}
		if err := ValidateProposal(proposal); err != nil {
			return integrityFromValidation(fmt.Sprintf("候选修订 %d 无效", expectedRevision), err)
		}
		previousTime = proposal.SubmittedAt
	}
	return nil
}

func validateReceivers(value *CoordinationCase) error {
	seen := make(map[string]struct{}, len(value.Receivers))
	for index, receiver := range value.Receivers {
		if receiver.CaseID != value.ID {
			return NewError(CodeIntegrity, fmt.Sprintf("第 %d 个受保护点归属错误", index+1))
		}
		if _, exists := seen[receiver.ID]; exists {
			return NewError(CodeIntegrity, "受保护点 ID 重复")
		}
		seen[receiver.ID] = struct{}{}
		if err := ValidateReceiver(receiver); err != nil {
			return integrityFromValidation(fmt.Sprintf("受保护点 %s 无效", receiver.ID), err)
		}
	}
	return nil
}

func validateAssessmentHistory(value *CoordinationCase) error {
	if value.CurrentAssessmentRevision < 0 || value.CurrentAssessmentRevision > len(value.Assessments) {
		return NewError(CodeIntegrity, "当前分析修订索引越界")
	}
	for index, assessment := range value.Assessments {
		expectedRevision := index + 1
		if assessment.CaseID != value.ID || assessment.Revision != expectedRevision {
			return NewError(CodeIntegrity, fmt.Sprintf("分析修订 %d 的归属或序号无效", expectedRevision))
		}
		if assessment.ID == "" || assessment.AlgorithmVersion == "" || assessment.InputDigest == "" {
			return NewError(CodeIntegrity, fmt.Sprintf("分析修订 %d 缺少不可变标识", expectedRevision))
		}
		if assessment.ProposalRevision < 1 || assessment.ProposalRevision > len(value.Proposals) {
			return NewError(CodeIntegrity, fmt.Sprintf("分析修订 %d 绑定了不存在的候选修订", expectedRevision))
		}
		if assessment.OverallOutcome != "pass" && assessment.OverallOutcome != "fail" {
			return NewError(CodeIntegrity, fmt.Sprintf("分析修订 %d 的总体结论无效", expectedRevision))
		}
		if len(assessment.PointResults) != len(value.Receivers) {
			return NewError(CodeIntegrity, fmt.Sprintf("分析修订 %d 的逐点结果数量不一致", expectedRevision))
		}
		if len(assessment.PointResults) == 0 {
			return NewError(CodeIntegrity, "分析修订不能没有逐点结果")
		}
		minimum, allPassed := assessment.PointResults[0].MarginDB, true
		for _, point := range assessment.PointResults {
			if point.ReceiverID == "" || len(point.Rules) == 0 {
				return NewError(CodeIntegrity, "逐点分析缺少接收点或规则依据")
			}
			if point.MarginDB < minimum {
				minimum = point.MarginDB
			}
			if !point.Passed {
				allPassed = false
			}
		}
		if !almostEqual(minimum, assessment.MinimumMarginDB) {
			return NewError(CodeIntegrity, "分析最小裕量与逐点结果不一致")
		}
		if (assessment.OverallOutcome == "pass") != allPassed {
			return NewError(CodeIntegrity, "分析总体结论与逐点结果不一致")
		}
	}
	return nil
}

func validateReviewHistory(value *CoordinationCase) error {
	seen := make(map[string]struct{}, len(value.Reviews))
	previousTime := value.CreatedAt
	for index, review := range value.Reviews {
		if review.ID == "" || review.CaseID != value.ID {
			return NewError(CodeIntegrity, fmt.Sprintf("第 %d 条复核决定标识或归属无效", index+1))
		}
		if _, exists := seen[review.ID]; exists {
			return NewError(CodeIntegrity, "复核决定 ID 重复")
		}
		seen[review.ID] = struct{}{}
		if review.AssessmentRevision < 1 || review.AssessmentRevision > len(value.Assessments) {
			return NewError(CodeIntegrity, "复核决定绑定了不存在的分析修订")
		}
		if review.DecidedAt.Before(previousTime) {
			return NewError(CodeIntegrity, "复核决定时间未按顺序记录")
		}
		if err := ValidateReview(review); err != nil {
			return integrityFromValidation("复核决定字段无效", err)
		}
		for index := range review.Findings {
			if review.Findings[index].Severity == "blocking" && review.Findings[index].ID == "" {
				return NewError(CodeIntegrity, "blocking 复核意见缺少稳定标识")
			}
		}
		previousTime = review.DecidedAt
	}
	return nil
}

func validateReviewResponses(value *CoordinationCase) error {
	seen := map[string]bool{}
	for _, response := range value.ReviewResponses {
		if response.CaseID != value.ID || response.ReviewID == "" || response.FindingID == "" || response.Responder == "" || response.Explanation == "" {
			return NewError(CodeIntegrity, "整改响应字段不完整")
		}
		key := response.ReviewID + "|" + response.FindingID
		if seen[key] {
			return NewError(CodeIntegrity, "整改响应重复")
		}
		seen[key] = true
		if response.ProposalRevision < 1 || response.ProposalRevision > len(value.Proposals) {
			return NewError(CodeIntegrity, "整改响应绑定了不存在的候选修订")
		}
	}
	return nil
}

func validateStateProjection(value *CoordinationCase) error {
	hasProposal := value.CurrentProposalRevision > 0
	hasReceivers := len(value.Receivers) > 0
	hasCurrentAssessment := value.CurrentAssessmentRevision > 0
	switch value.State {
	case StateAnalysisPending:
		if !hasProposal || !hasReceivers || hasCurrentAssessment {
			return NewError(CodeIntegrity, "analysis_pending 状态投影不完整")
		}
	case StateAnalyzed, StateUnderReview, StateApproved, StateFrozen, StateAuthorized:
		if !hasProposal || !hasReceivers || !hasCurrentAssessment {
			return NewError(CodeIntegrity, "当前状态缺少候选、保护点或分析修订")
		}
	}
	if value.State == StateApproved || value.State == StateFrozen || value.State == StateAuthorized {
		review, ok := value.LatestReview()
		if !ok || review.Decision != "approved" || review.AssessmentRevision != value.CurrentAssessmentRevision {
			return NewError(CodeIntegrity, "批准状态未绑定最新分析的 approved 决定")
		}
	}
	if value.State == StateFrozen || value.State == StateAuthorized {
		if value.Frozen == nil {
			return NewError(CodeIntegrity, "冻结状态缺少冻结记录")
		}
		valid, _, err := VerifyFrozen(value)
		if err != nil {
			return err
		}
		if !valid {
			return NewError(CodeIntegrity, "冻结摘要与聚合内容不一致")
		}
	} else if value.Frozen != nil {
		return NewError(CodeIntegrity, "未冻结状态包含冻结记录")
	}
	if value.State == StateAuthorized {
		if value.Authorization == nil {
			return NewError(CodeIntegrity, "authorized 状态缺少授权凭据")
		}
		valid, _, err := VerifyAuthorization(*value.Authorization)
		if err != nil {
			return err
		}
		if !valid {
			return NewError(CodeIntegrity, "授权凭据摘要无效")
		}
	} else if value.Authorization != nil {
		return NewError(CodeIntegrity, "非 authorized 状态包含授权凭据")
	}
	return nil
}

func VerifyFrozen(value *CoordinationCase) (bool, string, error) {
	if value.Frozen == nil {
		return false, "", NewError(CodeNotFound, "协调案尚未冻结")
	}
	if value.Frozen.ProposalRevision != value.CurrentProposalRevision || value.Frozen.AssessmentRevision != value.CurrentAssessmentRevision {
		return false, "", nil
	}
	var review ReviewDecision
	found := false
	for _, candidate := range value.Reviews {
		if candidate.ID == value.Frozen.ReviewID {
			review = candidate
			found = true
			break
		}
	}
	if !found || review.Decision != "approved" {
		return false, "", nil
	}
	computed, err := FrozenDigest(value, review)
	if err != nil {
		return false, "", err
	}
	return computed == value.Frozen.Digest, computed, nil
}

func integrityFromValidation(message string, err error) error {
	return &Error{Code: CodeIntegrity, Message: message, Cause: err}
}

func almostEqual(left, right float64) bool {
	difference := left - right
	if difference < 0 {
		difference = -difference
	}
	return difference <= 0.000001
}
