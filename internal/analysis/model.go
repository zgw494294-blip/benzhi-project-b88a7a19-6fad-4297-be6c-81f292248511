package analysis

import (
	"time"

	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/domain"
)

const AlgorithmVersion = "radio-interference-v1.0"

type Engine struct {
	Version string
}

type Input struct {
	CaseID       string
	AssessmentID string
	Revision     int
	Proposal     domain.TransmitterProposal
	Receivers    []domain.ProtectedReceiver
	CreatedAt    time.Time
}

type normalizedInput struct {
	AlgorithmVersion string                     `json:"algorithmVersion"`
	CaseID           string                     `json:"caseId"`
	Proposal         domain.TransmitterProposal `json:"proposal"`
	Receivers        []domain.ProtectedReceiver `json:"receivers"`
}

func NewEngine() *Engine {
	return &Engine{Version: AlgorithmVersion}
}
