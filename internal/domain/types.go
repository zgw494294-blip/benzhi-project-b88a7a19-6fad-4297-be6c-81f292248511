package domain

import "time"

type CaseState string

const (
	StateDraft            CaseState = "draft"
	StateAnalysisPending  CaseState = "analysis_pending"
	StateAnalyzed         CaseState = "analyzed"
	StateUnderReview      CaseState = "under_review"
	StateRevisionRequired CaseState = "revision_required"
	StateApproved         CaseState = "approved"
	StateFrozen           CaseState = "frozen"
	StateAuthorized       CaseState = "authorized"
)

type CoordinationCase struct {
	ID                        string                   `json:"id"`
	Title                     string                   `json:"title"`
	RegionCode                string                   `json:"regionCode"`
	Applicant                 string                   `json:"applicant"`
	State                     CaseState                `json:"state"`
	CurrentProposalRevision   int                      `json:"currentProposalRevision"`
	CurrentAssessmentRevision int                      `json:"currentAssessmentRevision"`
	Version                   int64                    `json:"version"`
	CreatedAt                 time.Time                `json:"createdAt"`
	UpdatedAt                 time.Time                `json:"updatedAt"`
	Proposals                 []TransmitterProposal    `json:"proposals"`
	Receivers                 []ProtectedReceiver      `json:"receivers"`
	Assessments               []InterferenceAssessment `json:"assessments"`
	Reviews                   []ReviewDecision         `json:"reviews"`
	ReviewResponses           []ReviewResponse         `json:"reviewResponses,omitempty"`
	Frozen                    *FrozenVersion           `json:"frozen,omitempty"`
	Authorization             *TrialAuthorization      `json:"authorization,omitempty"`
}

type TransmitterProposal struct {
	CaseID         string    `json:"caseId"`
	Revision       int       `json:"revision"`
	FrequencyHz    float64   `json:"frequencyHz"`
	BandwidthHz    float64   `json:"bandwidthHz"`
	EIRPDbm        float64   `json:"eirpDbm"`
	AntennaGainDbi float64   `json:"antennaGainDbi"`
	AntennaHeightM float64   `json:"antennaHeightM"`
	Latitude       float64   `json:"latitude"`
	Longitude      float64   `json:"longitude"`
	EmissionClass  string    `json:"emissionClass"`
	Rationale      string    `json:"rationale"`
	SubmittedAt    time.Time `json:"submittedAt"`
}

type ProtectedReceiver struct {
	ID                     string  `json:"id"`
	CaseID                 string  `json:"caseId"`
	Label                  string  `json:"label"`
	Latitude               float64 `json:"latitude"`
	Longitude              float64 `json:"longitude"`
	ReceiveFrequencyHz     float64 `json:"receiveFrequencyHz"`
	ProtectionThresholdDbm float64 `json:"protectionThresholdDbm"`
	AntennaGainDbi         float64 `json:"antennaGainDbi"`
	TerrainClass           string  `json:"terrainClass"`
	EvidenceRef            string  `json:"evidenceRef"`
}

type PointAssessment struct {
	ReceiverID              string   `json:"receiverId"`
	ReceiverLabel           string   `json:"receiverLabel"`
	DistanceKM              float64  `json:"distanceKm"`
	FrequencySeparationHz   float64  `json:"frequencySeparationHz"`
	FreeSpacePathLossDB     float64  `json:"freeSpacePathLossDb"`
	HeightCorrectionDB      float64  `json:"heightCorrectionDb"`
	TerrainCorrectionDB     float64  `json:"terrainCorrectionDb"`
	FrequencyRejectionDB    float64  `json:"frequencyRejectionDb"`
	ReceivedInterferenceDBm float64  `json:"receivedInterferenceDbm"`
	ThresholdDBm            float64  `json:"thresholdDbm"`
	MarginDB                float64  `json:"marginDb"`
	Passed                  bool     `json:"passed"`
	Rules                   []string `json:"rules"`
}

type InterferenceAssessment struct {
	ID               string            `json:"id"`
	CaseID           string            `json:"caseId"`
	Revision         int               `json:"revision"`
	ProposalRevision int               `json:"proposalRevision"`
	AlgorithmVersion string            `json:"algorithmVersion"`
	InputDigest      string            `json:"inputDigest"`
	PointResults     []PointAssessment `json:"pointResults"`
	OverallOutcome   string            `json:"overallOutcome"`
	MinimumMarginDB  float64           `json:"minimumMarginDb"`
	CreatedAt        time.Time         `json:"createdAt"`
}

type ReviewFinding struct {
	ID       string `json:"id,omitempty"`
	Item     string `json:"item"`
	Severity string `json:"severity"`
	Comment  string `json:"comment"`
}

type ReviewResponse struct {
	ID               string    `json:"id"`
	CaseID           string    `json:"caseId"`
	ReviewID         string    `json:"reviewId"`
	FindingID        string    `json:"findingId"`
	Responder        string    `json:"responder"`
	Explanation      string    `json:"explanation"`
	ProposalRevision int       `json:"proposalRevision"`
	RespondedAt      time.Time `json:"respondedAt"`
}

type ReviewDecision struct {
	ID                 string          `json:"id"`
	CaseID             string          `json:"caseId"`
	AssessmentRevision int             `json:"assessmentRevision"`
	Reviewer           string          `json:"reviewer"`
	Findings           []ReviewFinding `json:"findings"`
	Decision           string          `json:"decision"`
	Reason             string          `json:"reason"`
	DecidedAt          time.Time       `json:"decidedAt"`
}

type FrozenVersion struct {
	ProposalRevision   int       `json:"proposalRevision"`
	AssessmentRevision int       `json:"assessmentRevision"`
	ReviewID           string    `json:"reviewId"`
	Digest             string    `json:"digest"`
	FrozenBy           string    `json:"frozenBy"`
	FrozenAt           time.Time `json:"frozenAt"`
}

type TrialAuthorization struct {
	AuthorizationNo    string    `json:"authorizationNo"`
	CaseID             string    `json:"caseId"`
	FrozenDigest       string    `json:"frozenDigest"`
	ValidFrom          time.Time `json:"validFrom"`
	ValidUntil         time.Time `json:"validUntil"`
	Conditions         []string  `json:"conditions"`
	Issuer             string    `json:"issuer"`
	IssuedAt           time.Time `json:"issuedAt"`
	VerificationDigest string    `json:"verificationDigest"`
}

type AuditEntry struct {
	Sequence     int64          `json:"sequence"`
	CaseID       string         `json:"caseId"`
	Action       string         `json:"action"`
	Actor        string         `json:"actor"`
	CaseVersion  int64          `json:"caseVersion"`
	OccurredAt   time.Time      `json:"occurredAt"`
	Details      map[string]any `json:"details,omitempty"`
	PreviousHash string         `json:"previousHash"`
	Hash         string         `json:"hash"`
}
