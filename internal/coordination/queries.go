package coordination

import (
	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/domain"
	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/store"
	"math"
	"sort"
	"time"
)

func (s *Service) Preflight(caseID string, expectedVersion int64) (PreflightResult, error) {
	value, err := s.repository.Get(caseID)
	if err != nil {
		return PreflightResult{}, err
	}
	if err := value.EnsureVersion(expectedVersion); err != nil {
		return PreflightResult{}, err
	}
	result := PreflightResult{CheckedVersion: value.Version, Blocking: []domain.FieldViolation{}}
	if value.CurrentProposalRevision == 0 {
		result.Blocking = append(result.Blocking, domain.Violation("proposal", "required", "协调案缺少候选参数"))
	} else if proposal, ok := value.LatestProposal(); ok {
		if e := domain.ValidateProposal(proposal); e != nil {
			if de, ok := e.(*domain.Error); ok {
				result.Blocking = append(result.Blocking, de.Violations...)
			}
		}
	}
	if len(value.Receivers) == 0 {
		result.Blocking = append(result.Blocking, domain.Violation("receivers", "min_items", "协调案至少需要一个受保护点"))
	} else {
		for _, receiver := range value.Receivers {
			if e := domain.ValidateReceiver(receiver); e != nil {
				if de, ok := e.(*domain.Error); ok {
					for _, v := range de.Violations {
						v.Field = "receivers[" + receiver.ID + "]." + v.Field
						result.Blocking = append(result.Blocking, v)
					}
				}
			}
		}
	}
	result.ProposalRevision = value.CurrentProposalRevision
	if value.State != domain.StateDraft {
		result.Blocking = append(result.Blocking, domain.Violation("state", "state", "协调案必须处于 draft 状态"))
	}
	sort.SliceStable(result.Blocking, func(i, j int) bool { return result.Blocking[i].Field < result.Blocking[j].Field })
	result.Ready = len(result.Blocking) == 0
	return result, nil
}

func (s *Service) Remediation(caseID string) (RemediationResult, error) {
	value, err := s.repository.Get(caseID)
	if err != nil {
		return RemediationResult{}, err
	}
	assessment, ok := value.LatestAssessment()
	if !ok {
		return RemediationResult{}, domain.NewError(domain.CodeNotFound, "协调案尚无分析修订")
	}
	proposal, ok := value.LatestProposal()
	if !ok || proposal.Revision != assessment.ProposalRevision {
		return RemediationResult{}, domain.NewError(domain.CodeIntegrity, "分析与候选绑定不完整")
	}
	verification, err := s.engine.VerifyAssessment(assessment, proposal, value.Receivers)
	if err != nil {
		return RemediationResult{}, err
	}
	if !verification.Valid {
		return RemediationResult{}, domain.NewError(domain.CodeIntegrity, "最新分析无法通过完整性复算")
	}
	result := RemediationResult{CaseID: caseID, ProposalRevision: proposal.Revision, AssessmentRevision: assessment.Revision, AlgorithmVersion: assessment.AlgorithmVersion, InputDigest: assessment.InputDigest, OverallOutcome: assessment.OverallOutcome, Feasible: true, Points: []RemediationPoint{}}
	maxReduction := 0.0
	constraint := ""
	for _, point := range assessment.PointResults {
		if point.Passed {
			continue
		}
		reduction := round2(-point.MarginDB)
		suggested := round2(proposal.EIRPDbm - reduction)
		result.Points = append(result.Points, RemediationPoint{ReceiverID: point.ReceiverID, CurrentEIRPDbm: proposal.EIRPDbm, RequiredReductionDb: reduction, SuggestedEIRPDbm: suggested, MarginDb: point.MarginDB})
		if reduction > maxReduction {
			maxReduction = reduction
			constraint = point.ReceiverID
		}
	}
	if maxReduction == 0 {
		result.Reason = "总体结论已通过，无需降低功率"
		return result, nil
	}
	result.NeedsRemediation = true
	suggested := round2(proposal.EIRPDbm - maxReduction)
	result.SuggestedEIRPDbm = &suggested
	result.ConstrainingReceiverID = constraint
	if suggested < -30 {
		result.Feasible = false
		result.Reason = "建议 EIRP 低于领域允许下限 -30 dBm，当前参数不可行"
	}
	return result, nil
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

func (s *Service) CompareAssessments(caseID string, base, target int) (AssessmentComparison, error) {
	value, err := s.repository.Get(caseID)
	if err != nil {
		return AssessmentComparison{}, err
	}
	var ba, ta domain.InterferenceAssessment
	var bp, tp domain.TransmitterProposal
	var bok, tok bool
	for _, a := range value.Assessments {
		if a.Revision == base {
			ba = a
			bok = true
		}
		if a.Revision == target {
			ta = a
			tok = true
		}
	}
	if !bok || !tok {
		return AssessmentComparison{}, domain.NewError(domain.CodeNotFound, "分析修订不存在")
	}
	for _, p := range value.Proposals {
		if p.Revision == ba.ProposalRevision {
			bp = p
		}
		if p.Revision == ta.ProposalRevision {
			tp = p
		}
	}
	if bp.Revision == 0 || tp.Revision == 0 || ba.CaseID != caseID || ta.CaseID != caseID {
		return AssessmentComparison{}, domain.NewError(domain.CodeIntegrity, "分析修订绑定不完整")
	}
	bv, err := s.engine.VerifyAssessment(ba, bp, value.Receivers)
	if err != nil {
		return AssessmentComparison{}, err
	}
	tv, err := s.engine.VerifyAssessment(ta, tp, value.Receivers)
	if err != nil {
		return AssessmentComparison{}, err
	}
	if !bv.Valid || !tv.Valid {
		return AssessmentComparison{}, domain.NewError(domain.CodeIntegrity, "分析修订无法通过完整性复算")
	}
	changes := map[string]FieldChange{}
	if bp.FrequencyHz != tp.FrequencyHz {
		changes["frequencyHz"] = FieldChange{bp.FrequencyHz, tp.FrequencyHz}
	}
	if bp.BandwidthHz != tp.BandwidthHz {
		changes["bandwidthHz"] = FieldChange{bp.BandwidthHz, tp.BandwidthHz}
	}
	if bp.EIRPDbm != tp.EIRPDbm {
		changes["eirpDbm"] = FieldChange{bp.EIRPDbm, tp.EIRPDbm}
	}
	if bp.AntennaGainDbi != tp.AntennaGainDbi {
		changes["antennaGainDbi"] = FieldChange{bp.AntennaGainDbi, tp.AntennaGainDbi}
	}
	if bp.AntennaHeightM != tp.AntennaHeightM {
		changes["antennaHeightM"] = FieldChange{bp.AntennaHeightM, tp.AntennaHeightM}
	}
	if bp.Latitude != tp.Latitude {
		changes["latitude"] = FieldChange{bp.Latitude, tp.Latitude}
	}
	if bp.Longitude != tp.Longitude {
		changes["longitude"] = FieldChange{bp.Longitude, tp.Longitude}
	}
	by := map[string]domain.PointAssessment{}
	for _, p := range ba.PointResults {
		by[p.ReceiverID] = p
	}
	points := []PointComparison{}
	for _, p := range ta.PointResults {
		q, ok := by[p.ReceiverID]
		if !ok {
			continue
		}
		status := ""
		if q.Passed && !p.Passed {
			status = "passed_to_failed"
		}
		if !q.Passed && p.Passed {
			status = "failed_to_passed"
		}
		points = append(points, PointComparison{ReceiverID: p.ReceiverID, BaseMarginDb: q.MarginDB, TargetMarginDb: p.MarginDB, MarginDeltaDb: round2(p.MarginDB - q.MarginDB), BaseInterferenceDbm: q.ReceivedInterferenceDBm, TargetInterferenceDbm: p.ReceivedInterferenceDBm, StatusChange: status})
	}
	if len(points) > 0 {
		baseConstraint, targetConstraint := points[0].ReceiverID, points[0].ReceiverID
		baseMargin, targetMargin := points[0].BaseMarginDb, points[0].TargetMarginDb
		for _, point := range points[1:] {
			if point.BaseMarginDb < baseMargin {
				baseMargin, baseConstraint = point.BaseMarginDb, point.ReceiverID
			}
			if point.TargetMarginDb < targetMargin {
				targetMargin, targetConstraint = point.TargetMarginDb, point.ReceiverID
			}
		}
		if baseConstraint != targetConstraint {
			for i := range points {
				if points[i].ReceiverID == baseConstraint || points[i].ReceiverID == targetConstraint {
					if points[i].StatusChange == "" {
						points[i].StatusChange = "constraint_changed"
					} else {
						points[i].StatusChange += ",constraint_changed"
					}
				}
			}
		}
	}
	sort.Slice(points, func(i, j int) bool { return points[i].ReceiverID < points[j].ReceiverID })
	return AssessmentComparison{CaseID: caseID, BaseRevision: base, TargetRevision: target, ProposalChanges: changes, Points: points, BaseOutcome: ba.OverallOutcome, TargetOutcome: ta.OverallOutcome}, nil
}

func (s *Service) FreezePreflight(caseID string, expectedVersion int64) (FreezePreflight, error) {
	value, err := s.repository.Get(caseID)
	if err != nil {
		return FreezePreflight{}, err
	}
	if err := value.EnsureVersion(expectedVersion); err != nil {
		return FreezePreflight{}, err
	}
	r := FreezePreflight{CheckedVersion: value.Version, Blocking: []string{}, ReceiverCount: len(value.Receivers), ProposalRevision: value.CurrentProposalRevision, AssessmentRevision: value.CurrentAssessmentRevision}
	if value.State != domain.StateApproved {
		r.Blocking = append(r.Blocking, "协调案必须处于 approved 状态")
	}
	a, ok := value.LatestAssessment()
	if !ok {
		r.Blocking = append(r.Blocking, "缺少最新分析修订")
	} else {
		r.AssessmentRevision = a.Revision
		p, ok := value.LatestProposal()
		if !ok || p.Revision != a.ProposalRevision {
			r.Blocking = append(r.Blocking, "候选与分析修订未绑定")
		} else {
			v, e := s.engine.VerifyAssessment(a, p, value.Receivers)
			if e != nil {
				r.Blocking = append(r.Blocking, "分析复算执行失败: "+e.Error())
			} else if !v.Valid {
				if !v.AlgorithmSupported {
					r.Blocking = append(r.Blocking, "算法版本不受支持")
				}
				if !v.InputDigestValid {
					r.Blocking = append(r.Blocking, "输入摘要与分析记录不一致")
				}
				if !v.OutcomeValid {
					r.Blocking = append(r.Blocking, "总体结论与复算不一致")
				}
				if !v.PointResultsValid {
					r.Blocking = append(r.Blocking, "逐点计算结果与复算不一致")
				}
			}
		}
	}
	review, ok := value.LatestReview()
	if !ok || review.Decision != "approved" {
		r.Blocking = append(r.Blocking, "缺少 approved 复核决定")
	} else {
		r.ReviewID = review.ID
		if review.AssessmentRevision != value.CurrentAssessmentRevision {
			r.Blocking = append(r.Blocking, "复核决定未绑定最新分析")
		}
		for _, f := range review.Findings {
			if f.Severity == "blocking" {
				r.Blocking = append(r.Blocking, "存在 blocking 复核意见")
			}
		}
	}
	if len(r.Blocking) == 0 {
		digest, e := domain.FrozenDigest(value, review)
		if e != nil {
			r.Blocking = append(r.Blocking, e.Error())
		} else {
			r.ProspectiveFrozenDigest = digest
			r.Ready = true
		}
	}
	return r, nil
}

func (s *Service) GetCase(caseID string) (*domain.CoordinationCase, error) {
	return s.repository.Get(caseID)
}

func (s *Service) GetAnalysisBasis(caseID string) (AnalysisBasis, error) {
	value, err := s.repository.Get(caseID)
	if err != nil {
		return AnalysisBasis{}, err
	}
	assessment, ok := value.LatestAssessment()
	if !ok {
		return AnalysisBasis{}, domain.NewError(domain.CodeNotFound, "协调案尚无分析修订")
	}
	var proposal domain.TransmitterProposal
	found := false
	for _, candidate := range value.Proposals {
		if candidate.Revision == assessment.ProposalRevision {
			proposal = candidate
			found = true
			break
		}
	}
	if !found {
		return AnalysisBasis{}, domain.NewError(domain.CodeIntegrity, "分析绑定的候选修订不存在")
	}
	verification, err := s.engine.VerifyAssessment(assessment, proposal, value.Receivers)
	if err != nil {
		return AnalysisBasis{}, err
	}
	return AnalysisBasis{
		CaseID: caseID, Proposal: proposal, Receivers: append([]domain.ProtectedReceiver(nil), value.Receivers...),
		Assessment: assessment, InputVerified: verification.InputDigestValid,
		ComputationVerified: verification.Valid, ComputedDigest: verification.ComputedDigest, Verification: verification,
	}, nil
}

func (s *Service) GetAudit(caseID string) ([]domain.AuditEntry, error) {
	return s.repository.Audit(caseID)
}

func (s *Service) GetAuditFiltered(caseID string, filter store.AuditFilter) (store.AuditPage, error) {
	reader, ok := s.repository.(AuditReader)
	if !ok {
		entries, err := s.repository.Audit(caseID)
		return store.AuditPage{Entries: entries, ChainValid: err == nil, CheckedCount: len(entries)}, err
	}
	return reader.AuditFiltered(caseID, filter)
}

func (s *Service) GetReviewResponseViews(caseID string) ([]ReviewResponseView, error) {
	value, err := s.repository.Get(caseID)
	if err != nil {
		return nil, err
	}
	review, ok := value.LatestReview()
	if !ok {
		return []ReviewResponseView{}, nil
	}
	by := map[string]domain.ReviewResponse{}
	for _, r := range value.ReviewResponses {
		if r.ReviewID == review.ID && r.ProposalRevision == value.CurrentProposalRevision {
			by[r.FindingID] = r
		}
	}
	result := make([]ReviewResponseView, 0, len(review.Findings))
	for _, f := range review.Findings {
		v := ReviewResponseView{FindingID: f.ID, Item: f.Item, Severity: f.Severity, Status: "unresponded"}
		if r, ok := by[f.ID]; ok {
			v.Status = "responded"
			v.Responder = r.Responder
			v.RespondedAt = r.RespondedAt
			v.Explanation = r.Explanation
			v.ProposalRevision = r.ProposalRevision
		}
		result = append(result, v)
	}
	return result, nil
}

func (s *Service) GetAuthorization(caseID string) (domain.TrialAuthorization, error) {
	value, err := s.repository.Get(caseID)
	if err != nil {
		return domain.TrialAuthorization{}, err
	}
	if value.Authorization == nil {
		return domain.TrialAuthorization{}, domain.NewError(domain.CodeNotFound, "协调案尚未签发试播授权")
	}
	return *value.Authorization, nil
}

func (s *Service) VerifyAuthorization(caseID, authorizationNo string) (AuthorizationVerification, error) {
	return s.VerifyAuthorizationAt(caseID, authorizationNo, s.clock())
}

func (s *Service) VerifyAuthorizationAt(caseID, authorizationNo string, at time.Time) (AuthorizationVerification, error) {
	value, err := s.repository.Get(caseID)
	if err != nil {
		return AuthorizationVerification{}, err
	}
	if value.Authorization == nil || value.Frozen == nil {
		return AuthorizationVerification{}, domain.NewError(domain.CodeNotFound, "协调案没有可验证的试播授权")
	}
	authorization := *value.Authorization
	if authorizationNo != "" && authorization.AuthorizationNo != authorizationNo {
		return AuthorizationVerification{AuthorizationNo: authorizationNo, CaseID: caseID, Valid: false, Reason: "授权编号与协调案记录不匹配"}, nil
	}
	credentialValid, computed, err := domain.VerifyAuthorization(authorization)
	if err != nil {
		return AuthorizationVerification{}, err
	}
	frozenContentValid, _, err := domain.VerifyFrozen(value)
	if err != nil {
		return AuthorizationVerification{}, err
	}
	frozenValid := frozenContentValid && authorization.FrozenDigest == value.Frozen.Digest
	reason := "授权凭据及冻结绑定均完整"
	if !credentialValid {
		reason = "授权凭据摘要不匹配"
	}
	if credentialValid && !frozenValid {
		reason = "授权绑定的冻结摘要不匹配"
	}
	result := AuthorizationVerification{
		AuthorizationNo: authorization.AuthorizationNo, CaseID: caseID,
		Valid: credentialValid && frozenValid, FrozenDigestValid: frozenValid,
		CredentialValid: credentialValid, ComputedDigest: computed, Reason: reason,
		CheckedAt: at.UTC(), ValidFrom: authorization.ValidFrom, ValidUntil: authorization.ValidUntil, Conditions: append([]string(nil), authorization.Conditions...), OperationallyValid: false,
	}
	if !result.Valid {
		result.TimeState = "invalid"
	} else if at.Before(authorization.ValidFrom) {
		result.TimeState = "pending"
	} else if !at.Before(authorization.ValidUntil) {
		result.TimeState = "expired"
	} else {
		result.TimeState = "effective"
		result.OperationallyValid = true
	}
	return result, nil
}
