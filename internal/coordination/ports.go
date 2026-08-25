package coordination

import (
	"time"

	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/analysis"
	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/domain"
	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/store"
)

type Repository interface {
	Create(value *domain.CoordinationCase, action, actor string, details map[string]any) (*domain.CoordinationCase, error)
	Update(caseID string, expectedVersion int64, action, actor string, details map[string]any, idem *store.IdempotencyRequest, mutate store.Mutation) (*domain.CoordinationCase, bool, error)
	Get(caseID string) (*domain.CoordinationCase, error)
	Audit(caseID string) ([]domain.AuditEntry, error)
}

type AuditReader interface {
	AuditFiltered(caseID string, filter store.AuditFilter) (store.AuditPage, error)
}

type AssessmentEngine interface {
	Evaluate(input analysis.Input) (domain.InterferenceAssessment, error)
	VerifyInput(assessment domain.InterferenceAssessment, proposal domain.TransmitterProposal, receivers []domain.ProtectedReceiver) (bool, string, error)
	VerifyAssessment(assessment domain.InterferenceAssessment, proposal domain.TransmitterProposal, receivers []domain.ProtectedReceiver) (analysis.VerificationReport, error)
}

type Clock func() time.Time
type IDGenerator func(prefix string) string
