package coordination

import (
	"context"
	"strings"

	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/analysis"
	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/domain"
)

func (s *Service) RunAnalysisContext(ctx context.Context, caseID string, command VersionedCommand) (*domain.CoordinationCase, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := requireActor(command.Actor); err != nil {
		return nil, err
	}
	value, _, err := s.repository.Update(caseID, command.ExpectedVersion, "assessment_completed", strings.TrimSpace(command.Actor), nil, nil, func(value *domain.CoordinationCase) error {
		// 应用事务等待阶段：取得当前协调案状态后再次校验取消信号，避免在等待期间被取消的请求继续推进状态。
		if err := ctx.Err(); err != nil {
			return err
		}
		proposal, ok := value.LatestProposal()
		if !ok {
			return domain.NewError(domain.CodeIntegrity, "找不到最新候选修订")
		}
		assessment, err := s.engine.Evaluate(analysis.Input{
			CaseID: value.ID, AssessmentID: s.newID("assessment"), Revision: len(value.Assessments) + 1,
			Proposal: proposal, Receivers: value.Receivers, CreatedAt: s.clock(),
		})
		if err != nil {
			return err
		}
		// 分析引擎计算阶段：评估完成后、附加分析结果前再次校验取消信号，确保取消的请求不附加结果、不推进版本。
		if err := ctx.Err(); err != nil {
			return err
		}
		return value.AttachAssessment(assessment, s.clock())
	})
	return value, err
}

func (s *Service) RunAnalysis(caseID string, command VersionedCommand) (*domain.CoordinationCase, error) {
	if err := requireActor(command.Actor); err != nil {
		return nil, err
	}
	value, _, err := s.repository.Update(caseID, command.ExpectedVersion, "assessment_completed", strings.TrimSpace(command.Actor), nil, nil, func(value *domain.CoordinationCase) error {
		proposal, ok := value.LatestProposal()
		if !ok {
			return domain.NewError(domain.CodeIntegrity, "找不到最新候选修订")
		}
		assessment, err := s.engine.Evaluate(analysis.Input{
			CaseID: value.ID, AssessmentID: s.newID("assessment"), Revision: len(value.Assessments) + 1,
			Proposal: proposal, Receivers: value.Receivers, CreatedAt: s.clock(),
		})
		if err != nil {
			return err
		}
		return value.AttachAssessment(assessment, s.clock())
	})
	return value, err
}

func (s *Service) SubmitForReview(caseID string, command VersionedCommand) (*domain.CoordinationCase, error) {
	if err := requireActor(command.Actor); err != nil {
		return nil, err
	}
	value, _, err := s.repository.Update(caseID, command.ExpectedVersion, "review_submitted", strings.TrimSpace(command.Actor), nil, nil, func(value *domain.CoordinationCase) error {
		return value.SubmitForReview(s.clock())
	})
	return value, err
}

func (s *Service) AddReviewResponse(caseID string, command ReviewResponseCommand) (*domain.CoordinationCase, error) {
	if err := requireActor(command.Responder); err != nil {
		return nil, err
	}
	response := domain.ReviewResponse{ID: s.newID("response"), FindingID: command.FindingID, Responder: strings.TrimSpace(command.Responder), Explanation: strings.TrimSpace(command.Explanation)}
	details := map[string]any{"findingId": command.FindingID}
	returnValue, _, err := s.repository.Update(caseID, command.ExpectedVersion, "review_response_added", response.Responder, details, nil, func(value *domain.CoordinationCase) error {
		response.ProposalRevision = value.CurrentProposalRevision
		if command.ProposalRevision > 0 && command.ProposalRevision != value.CurrentProposalRevision {
			return domain.NewError(domain.CodeStateConflict, "整改响应绑定的候选修订不是当前修订")
		}
		if command.ReviewID != "" {
			latest, _ := value.LatestReview()
			if latest.ID != command.ReviewID {
				return domain.NewError(domain.CodeStateConflict, "整改响应未绑定上一轮退回复核")
			}
		}
		details["proposalRevision"] = value.CurrentProposalRevision
		return value.AddReviewResponse(response, s.clock())
	})
	return returnValue, err
}

func (s *Service) DecideReview(caseID string, command ReviewCommand) (*domain.CoordinationCase, error) {
	if err := requireActor(command.Reviewer); err != nil {
		return nil, err
	}
	review := domain.ReviewDecision{
		ID: s.newID("review"), AssessmentRevision: command.AssessmentRevision, Reviewer: strings.TrimSpace(command.Reviewer),
		Findings: append([]domain.ReviewFinding(nil), command.Findings...), Decision: command.Decision, Reason: command.Reason,
	}
	value, _, err := s.repository.Update(caseID, command.ExpectedVersion, "review_decided", review.Reviewer, map[string]any{"decision": command.Decision, "assessmentRevision": command.AssessmentRevision}, nil, func(value *domain.CoordinationCase) error {
		return value.DecideReview(review, s.clock())
	})
	return value, err
}

func (s *Service) Freeze(caseID string, command FreezeCommand) (*domain.CoordinationCase, error) {
	if err := requireActor(command.FrozenBy); err != nil {
		return nil, err
	}
	value, _, err := s.repository.Update(caseID, command.ExpectedVersion, "case_frozen", strings.TrimSpace(command.FrozenBy), nil, nil, func(value *domain.CoordinationCase) error {
		assessment, ok := value.LatestAssessment()
		if !ok {
			return domain.NewError(domain.CodeIntegrity, "缺少最新分析修订")
		}
		proposal, ok := value.LatestProposal()
		if !ok {
			return domain.NewError(domain.CodeIntegrity, "缺少最新候选修订")
		}
		verification, verifyErr := s.engine.VerifyAssessment(assessment, proposal, value.Receivers)
		if verifyErr != nil {
			return verifyErr
		}
		if !verification.Valid {
			return domain.NewError(domain.CodeIntegrity, "冻结前分析复算不一致")
		}
		return value.Freeze(command.FrozenBy, s.clock())
	})
	return value, err
}

func (s *Service) IssueAuthorization(caseID string, command AuthorizationCommand) (*domain.CoordinationCase, error) {
	if err := requireActor(command.Issuer); err != nil {
		return nil, err
	}
	authorization := domain.TrialAuthorization{
		AuthorizationNo: s.newID("TA"), ValidFrom: command.ValidFrom, ValidUntil: command.ValidUntil,
		Conditions: append([]string(nil), command.Conditions...), Issuer: strings.TrimSpace(command.Issuer),
	}
	value, _, err := s.repository.Update(caseID, command.ExpectedVersion, "authorization_issued", authorization.Issuer, map[string]any{"authorizationNo": authorization.AuthorizationNo}, nil, func(value *domain.CoordinationCase) error {
		return value.Authorize(authorization, s.clock())
	})
	return value, err
}
