package domain

import (
	"fmt"
	"strings"
	"time"
)

func NewCase(id, title, regionCode, applicant string, now time.Time) (*CoordinationCase, error) {
	if err := ValidateNewCase(title, regionCode, applicant); err != nil {
		return nil, err
	}
	if strings.TrimSpace(id) == "" {
		return nil, ValidationError(Violation("id", "required", "协调案 ID 不能为空"))
	}
	now = now.UTC()
	return &CoordinationCase{
		ID: id, Title: strings.TrimSpace(title), RegionCode: strings.TrimSpace(regionCode),
		Applicant: strings.TrimSpace(applicant), State: StateDraft, Version: 1,
		CreatedAt: now, UpdatedAt: now, Proposals: []TransmitterProposal{},
		Receivers: []ProtectedReceiver{}, Assessments: []InterferenceAssessment{}, Reviews: []ReviewDecision{},
		ReviewResponses: []ReviewResponse{},
	}, nil
}

// AddReceivers validates the complete batch before changing the aggregate.
func (c *CoordinationCase) AddReceivers(receivers []ProtectedReceiver, now time.Time) error {
	if c.State != StateDraft || len(c.Assessments) > 0 {
		return NewError(CodeStateConflict, "仅 draft 且尚未分析的协调案允许批量登记受保护点")
	}
	if len(receivers) == 0 {
		return ValidationError(Violation("receivers", "min_items", "至少需要一个受保护点"))
	}
	violations := make([]FieldViolation, 0)
	seen := make(map[string]struct{}, len(c.Receivers)+len(receivers))
	for _, existing := range c.Receivers {
		seen[existing.ID] = struct{}{}
	}
	for i, receiver := range receivers {
		receiver.CaseID = c.ID
		if err := ValidateReceiver(receiver); err != nil {
			if typed, ok := err.(*Error); ok {
				for _, v := range typed.Violations {
					v.Field = fmt.Sprintf("receivers[%d].%s", i, v.Field)
					violations = append(violations, v)
				}
			}
		}
		if _, exists := seen[receiver.ID]; exists && strings.TrimSpace(receiver.ID) != "" {
			violations = append(violations, Violation(fmt.Sprintf("receivers[%d].id", i), "duplicate", "受保护点 ID 在协调案或批次内重复"))
		}
		if strings.TrimSpace(receiver.ID) != "" {
			seen[receiver.ID] = struct{}{}
		}
	}
	if len(violations) > 0 {
		return ValidationError(violations...)
	}
	for _, receiver := range receivers {
		receiver.CaseID = c.ID
		c.Receivers = append(c.Receivers, receiver)
	}
	c.touch(now)
	return nil
}

func (c *CoordinationCase) EnsureVersion(expected int64) error {
	if expected <= 0 {
		return ValidationError(Violation("expectedVersion", "positive", "expectedVersion 必须为正整数"))
	}
	if c.Version != expected {
		return NewError(CodeVersionConflict, fmt.Sprintf("版本冲突：当前版本为 %d，提交版本为 %d", c.Version, expected))
	}
	return nil
}

func (c *CoordinationCase) ReplaceProposal(proposal TransmitterProposal, now time.Time) error {
	if c.State != StateDraft && c.State != StateRevisionRequired {
		return NewError(CodeStateConflict, "仅 draft 或 revision_required 状态允许登记候选参数")
	}
	proposal.CaseID = c.ID
	proposal.Revision = c.CurrentProposalRevision + 1
	proposal.SubmittedAt = now.UTC()
	if err := ValidateProposal(proposal); err != nil {
		return err
	}
	c.Proposals = append(c.Proposals, proposal)
	c.CurrentProposalRevision = proposal.Revision
	c.CurrentAssessmentRevision = 0
	c.State = StateDraft
	c.touch(now)
	return nil
}

func (c *CoordinationCase) AddReceiver(receiver ProtectedReceiver, now time.Time) error {
	if c.State != StateDraft && c.State != StateRevisionRequired {
		return NewError(CodeStateConflict, "当前状态不允许维护受保护点")
	}
	if len(c.Assessments) > 0 {
		return NewError(CodeStateConflict, "首次分析后受保护点集合已成为审计依据，不允许改写")
	}
	receiver.CaseID = c.ID
	if err := ValidateReceiver(receiver); err != nil {
		return err
	}
	for _, existing := range c.Receivers {
		if existing.ID == receiver.ID {
			return NewError(CodeAlreadyExists, "受保护点 ID 已存在")
		}
	}
	c.Receivers = append(c.Receivers, receiver)
	c.touch(now)
	return nil
}

func (c *CoordinationCase) ReplaceReceiver(receiverID string, receiver ProtectedReceiver, now time.Time) error {
	if c.State != StateDraft {
		return NewError(CodeStateConflict, "仅 draft 状态允许修订受保护点")
	}
	if len(c.Assessments) > 0 {
		return NewError(CodeStateConflict, "首次分析后受保护点集合已成为审计依据，不允许改写")
	}
	index := -1
	for candidateIndex, existing := range c.Receivers {
		if existing.ID == receiverID {
			index = candidateIndex
			break
		}
	}
	if index < 0 {
		return NewError(CodeNotFound, "受保护点不存在")
	}
	receiver.ID = receiverID
	receiver.CaseID = c.ID
	if err := ValidateReceiver(receiver); err != nil {
		return err
	}
	c.Receivers[index] = receiver
	c.touch(now)
	return nil
}

func (c *CoordinationCase) SubmitForAnalysis(now time.Time) error {
	if c.State != StateDraft {
		return NewError(CodeStateConflict, "仅 draft 状态允许提交分析候选")
	}
	if c.CurrentProposalRevision == 0 || len(c.Receivers) == 0 {
		return NewError(CodeStateConflict, "协调案必须包含候选参数和至少一个受保护点")
	}
	proposal, ok := c.LatestProposal()
	if !ok {
		return NewError(CodeIntegrity, "候选修订索引无效")
	}
	if err := ValidateProposal(proposal); err != nil {
		return err
	}
	for _, receiver := range c.Receivers {
		if err := ValidateReceiver(receiver); err != nil {
			return err
		}
	}
	c.State = StateAnalysisPending
	c.touch(now)
	return nil
}

func (c *CoordinationCase) AttachAssessment(assessment InterferenceAssessment, now time.Time) error {
	if c.State != StateAnalysisPending {
		return NewError(CodeStateConflict, "仅 analysis_pending 状态允许生成分析")
	}
	if assessment.ProposalRevision != c.CurrentProposalRevision {
		return NewError(CodeStateConflict, "分析未绑定最新候选修订")
	}
	if assessment.InputDigest == "" || len(assessment.PointResults) != len(c.Receivers) {
		return NewError(CodeIntegrity, "分析结果不完整")
	}
	assessment.CaseID = c.ID
	assessment.Revision = len(c.Assessments) + 1
	assessment.CreatedAt = now.UTC()
	c.Assessments = append(c.Assessments, assessment)
	c.CurrentAssessmentRevision = assessment.Revision
	c.State = StateAnalyzed
	c.touch(now)
	return nil
}

func (c *CoordinationCase) SubmitForReview(now time.Time) error {
	if c.State != StateAnalyzed {
		return NewError(CodeStateConflict, "仅 analyzed 状态允许送交技术复核")
	}
	assessment, ok := c.LatestAssessment()
	if !ok || assessment.ProposalRevision != c.CurrentProposalRevision {
		return NewError(CodeStateConflict, "最新分析与候选修订不一致")
	}
	if gaps := c.ReviewResponseGaps(); len(gaps) > 0 {
		violations := make([]FieldViolation, 0, len(gaps))
		for _, id := range gaps {
			violations = append(violations, Violation("reviewResponses."+id, "required", "blocking 复核意见尚未闭环"))
		}
		return ValidationError(violations...)
	}
	c.State = StateUnderReview
	c.touch(now)
	return nil
}

func (c *CoordinationCase) DecideReview(review ReviewDecision, now time.Time) error {
	if c.State != StateUnderReview {
		return NewError(CodeStateConflict, "仅 under_review 状态允许记录复核决定")
	}
	if review.AssessmentRevision != c.CurrentAssessmentRevision {
		return NewError(CodeStateConflict, "复核决定未绑定最新分析修订")
	}
	review.CaseID = c.ID
	review.DecidedAt = now.UTC()
	for index := range review.Findings {
		if strings.TrimSpace(review.Findings[index].ID) == "" {
			review.Findings[index].ID = fmt.Sprintf("%s-f%03d", review.ID, index+1)
		}
	}
	if err := ValidateReview(review); err != nil {
		return err
	}
	if review.Decision == "approved" {
		assessment, ok := c.LatestAssessment()
		if !ok || assessment.OverallOutcome != "pass" {
			return NewError(CodeStateConflict, "最新干扰分析未通过时不能批准")
		}
		for _, finding := range review.Findings {
			if finding.Severity == "blocking" {
				return NewError(CodeStateConflict, "存在 blocking 意见时不能批准")
			}
		}
		c.State = StateApproved
	} else {
		c.State = StateRevisionRequired
	}
	c.Reviews = append(c.Reviews, review)
	c.touch(now)
	return nil
}

func (c *CoordinationCase) AddReviewResponse(response ReviewResponse, now time.Time) error {
	if c.State != StateRevisionRequired && c.State != StateDraft {
		return NewError(CodeStateConflict, "仅 revision_required 或已提交新候选的 draft 状态允许提交整改响应")
	}
	if strings.TrimSpace(response.Responder) == "" || strings.TrimSpace(response.Explanation) == "" {
		return ValidationError(Violation("explanation", "required", "整改说明不能为空"))
	}
	review, ok := c.LatestReview()
	if !ok || review.Decision != "changes_requested" {
		return NewError(CodeStateConflict, "不存在可响应的退回复核意见")
	}
	if len(c.Proposals) == 0 || c.CurrentProposalRevision <= review.AssessmentRevision {
		return NewError(CodeStateConflict, "整改响应必须绑定退回后的新候选修订")
	}
	response.ReviewID = review.ID
	response.CaseID = c.ID
	found := false
	blocking := false
	for _, finding := range review.Findings {
		if finding.ID == response.FindingID {
			found = true
			blocking = finding.Severity == "blocking"
			break
		}
	}
	if !found {
		return NewError(CodeNotFound, "复核意见不存在")
	}
	if !blocking {
		return NewError(CodeStateConflict, "仅 blocking 意见需要整改响应")
	}
	if response.ProposalRevision != c.CurrentProposalRevision {
		return NewError(CodeStateConflict, "整改响应必须绑定当前候选修订")
	}
	for _, existing := range c.ReviewResponses {
		if existing.ReviewID == response.ReviewID && existing.FindingID == response.FindingID {
			return NewError(CodeAlreadyExists, "该复核意见已经响应")
		}
	}
	response.RespondedAt = now.UTC()
	c.ReviewResponses = append(c.ReviewResponses, response)
	c.touch(now)
	return nil
}

func (c *CoordinationCase) ReviewResponseGaps() []string {
	review, ok := c.LatestReview()
	if !ok || review.Decision != "changes_requested" {
		return nil
	}
	responses := map[string]bool{}
	for _, response := range c.ReviewResponses {
		if response.ReviewID == review.ID && response.ProposalRevision == c.CurrentProposalRevision {
			responses[response.FindingID] = true
		}
	}
	gaps := []string{}
	for _, finding := range review.Findings {
		if finding.Severity == "blocking" && !responses[finding.ID] {
			gaps = append(gaps, finding.ID)
		}
	}
	return gaps
}

func (c *CoordinationCase) Freeze(frozenBy string, now time.Time) error {
	if c.State != StateApproved {
		return NewError(CodeStateConflict, "仅 approved 状态允许冻结")
	}
	if strings.TrimSpace(frozenBy) == "" {
		return ValidationError(Violation("frozenBy", "required", "冻结操作人不能为空"))
	}
	assessment, ok := c.LatestAssessment()
	if !ok || assessment.ProposalRevision != c.CurrentProposalRevision {
		return NewError(CodeIntegrity, "分析修订与候选修订不一致")
	}
	review, ok := c.LatestReview()
	if !ok || review.Decision != "approved" || review.AssessmentRevision != assessment.Revision {
		return NewError(CodeIntegrity, "批准决定与最新分析修订不一致")
	}
	digest, err := FrozenDigest(c, review)
	if err != nil {
		return err
	}
	c.Frozen = &FrozenVersion{ProposalRevision: c.CurrentProposalRevision, AssessmentRevision: c.CurrentAssessmentRevision, ReviewID: review.ID, Digest: digest, FrozenBy: strings.TrimSpace(frozenBy), FrozenAt: now.UTC()}
	c.State = StateFrozen
	c.touch(now)
	return nil
}

func (c *CoordinationCase) Authorize(authorization TrialAuthorization, now time.Time) error {
	if c.State != StateFrozen || c.Frozen == nil {
		return NewError(CodeStateConflict, "仅 frozen 状态允许签发试播授权")
	}
	if c.Authorization != nil {
		return NewError(CodeStateConflict, "试播授权已经签发且不可变")
	}
	authorization.CaseID = c.ID
	authorization.FrozenDigest = c.Frozen.Digest
	authorization.IssuedAt = now.UTC()
	if err := ValidateAuthorization(authorization); err != nil {
		return err
	}
	digest, err := AuthorizationDigest(authorization)
	if err != nil {
		return err
	}
	authorization.VerificationDigest = digest
	c.Authorization = &authorization
	c.State = StateAuthorized
	c.touch(now)
	return nil
}

func (c *CoordinationCase) LatestProposal() (TransmitterProposal, bool) {
	for i := len(c.Proposals) - 1; i >= 0; i-- {
		if c.Proposals[i].Revision == c.CurrentProposalRevision {
			return c.Proposals[i], true
		}
	}
	return TransmitterProposal{}, false
}

func (c *CoordinationCase) LatestAssessment() (InterferenceAssessment, bool) {
	for i := len(c.Assessments) - 1; i >= 0; i-- {
		if c.Assessments[i].Revision == c.CurrentAssessmentRevision {
			return c.Assessments[i], true
		}
	}
	return InterferenceAssessment{}, false
}

func (c *CoordinationCase) LatestReview() (ReviewDecision, bool) {
	if len(c.Reviews) == 0 {
		return ReviewDecision{}, false
	}
	return c.Reviews[len(c.Reviews)-1], true
}

func (c *CoordinationCase) touch(now time.Time) {
	c.Version++
	c.UpdatedAt = now.UTC()
}
