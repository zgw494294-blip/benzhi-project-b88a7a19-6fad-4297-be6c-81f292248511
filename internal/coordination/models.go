package coordination

import (
	"time"

	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/analysis"
	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/domain"
)

type CreateCaseCommand struct {
	Title      string `json:"title"`
	RegionCode string `json:"regionCode"`
	Applicant  string `json:"applicant"`
	Actor      string `json:"actor"`
}

type ProposalCommand struct {
	ExpectedVersion int64   `json:"expectedVersion"`
	FrequencyHz     float64 `json:"frequencyHz"`
	BandwidthHz     float64 `json:"bandwidthHz"`
	EIRPDbm         float64 `json:"eirpDbm"`
	AntennaGainDbi  float64 `json:"antennaGainDbi"`
	AntennaHeightM  float64 `json:"antennaHeightM"`
	Latitude        float64 `json:"latitude"`
	Longitude       float64 `json:"longitude"`
	EmissionClass   string  `json:"emissionClass"`
	Rationale       string  `json:"rationale"`
	Actor           string  `json:"actor"`
}

type ReceiverCommand struct {
	ExpectedVersion        int64   `json:"expectedVersion"`
	ID                     string  `json:"id"`
	Label                  string  `json:"label"`
	Latitude               float64 `json:"latitude"`
	Longitude              float64 `json:"longitude"`
	ReceiveFrequencyHz     float64 `json:"receiveFrequencyHz"`
	ProtectionThresholdDbm float64 `json:"protectionThresholdDbm"`
	AntennaGainDbi         float64 `json:"antennaGainDbi"`
	TerrainClass           string  `json:"terrainClass"`
	EvidenceRef            string  `json:"evidenceRef"`
	Actor                  string  `json:"actor"`
}

type BatchReceiverCommand struct {
	ExpectedVersion int64             `json:"expectedVersion"`
	Actor           string            `json:"actor"`
	Receivers       []ReceiverCommand `json:"receivers"`
}
type PreflightResult struct {
	CheckedVersion   int64                   `json:"checkedVersion"`
	Ready            bool                    `json:"ready"`
	Blocking         []domain.FieldViolation `json:"blocking"`
	ProposalRevision int                     `json:"proposalRevision"`
}
type RemediationPoint struct {
	ReceiverID          string  `json:"receiverId"`
	CurrentEIRPDbm      float64 `json:"currentEirpDbm"`
	RequiredReductionDb float64 `json:"requiredReductionDb"`
	SuggestedEIRPDbm    float64 `json:"suggestedEirpDbm"`
	MarginDb            float64 `json:"marginDb"`
}
type RemediationResult struct {
	CaseID                 string             `json:"caseId"`
	ProposalRevision       int                `json:"proposalRevision"`
	AssessmentRevision     int                `json:"assessmentRevision"`
	AlgorithmVersion       string             `json:"algorithmVersion"`
	InputDigest            string             `json:"inputDigest"`
	OverallOutcome         string             `json:"overallOutcome"`
	NeedsRemediation       bool               `json:"needsRemediation"`
	Feasible               bool               `json:"feasible"`
	SuggestedEIRPDbm       *float64           `json:"suggestedEirpDbm,omitempty"`
	ConstrainingReceiverID string             `json:"constrainingReceiverId,omitempty"`
	Points                 []RemediationPoint `json:"points"`
	Reason                 string             `json:"reason,omitempty"`
}
type FieldChange struct {
	Before any `json:"before"`
	After  any `json:"after"`
}
type PointComparison struct {
	ReceiverID            string  `json:"receiverId"`
	BaseMarginDb          float64 `json:"baseMarginDb"`
	TargetMarginDb        float64 `json:"targetMarginDb"`
	MarginDeltaDb         float64 `json:"marginDeltaDb"`
	BaseInterferenceDbm   float64 `json:"baseInterferenceDbm"`
	TargetInterferenceDbm float64 `json:"targetInterferenceDbm"`
	StatusChange          string  `json:"statusChange,omitempty"`
}
type AssessmentComparison struct {
	CaseID          string                 `json:"caseId"`
	BaseRevision    int                    `json:"baseRevision"`
	TargetRevision  int                    `json:"targetRevision"`
	ProposalChanges map[string]FieldChange `json:"proposalChanges"`
	Points          []PointComparison      `json:"points"`
	BaseOutcome     string                 `json:"baseOutcome"`
	TargetOutcome   string                 `json:"targetOutcome"`
}
type FreezePreflight struct {
	CheckedVersion          int64    `json:"checkedVersion"`
	Ready                   bool     `json:"ready"`
	Blocking                []string `json:"blocking"`
	ProposalRevision        int      `json:"proposalRevision"`
	AssessmentRevision      int      `json:"assessmentRevision"`
	ReviewID                string   `json:"reviewId"`
	ReceiverCount           int      `json:"receiverCount"`
	ProspectiveFrozenDigest string   `json:"prospectiveFrozenDigest,omitempty"`
}
type ReviewResponseCommand struct {
	ExpectedVersion  int64  `json:"expectedVersion"`
	FindingID        string `json:"findingId"`
	ProposalRevision int    `json:"proposalRevision,omitempty"`
	ReviewID         string `json:"reviewId,omitempty"`
	Explanation      string `json:"explanation"`
	Responder        string `json:"responder"`
}
type ReviewResponseView struct {
	FindingID        string    `json:"findingId"`
	Item             string    `json:"item"`
	Severity         string    `json:"severity"`
	Status           string    `json:"status"`
	Responder        string    `json:"responder,omitempty"`
	RespondedAt      time.Time `json:"respondedAt,omitempty"`
	Explanation      string    `json:"explanation,omitempty"`
	ProposalRevision int       `json:"proposalRevision,omitempty"`
}

type VersionedCommand struct {
	ExpectedVersion int64  `json:"expectedVersion"`
	Actor           string `json:"actor"`
}

type SubmitCommand struct {
	ExpectedVersion int64  `json:"expectedVersion"`
	IdempotencyKey  string `json:"idempotencyKey"`
	Actor           string `json:"actor"`
}

type ReviewCommand struct {
	ExpectedVersion    int64                  `json:"expectedVersion"`
	AssessmentRevision int                    `json:"assessmentRevision"`
	Reviewer           string                 `json:"reviewer"`
	Findings           []domain.ReviewFinding `json:"findings"`
	Decision           string                 `json:"decision"`
	Reason             string                 `json:"reason"`
}

type FreezeCommand struct {
	ExpectedVersion int64  `json:"expectedVersion"`
	FrozenBy        string `json:"frozenBy"`
}

type AuthorizationCommand struct {
	ExpectedVersion int64     `json:"expectedVersion"`
	ValidFrom       time.Time `json:"validFrom"`
	ValidUntil      time.Time `json:"validUntil"`
	Conditions      []string  `json:"conditions"`
	Issuer          string    `json:"issuer"`
}

type AnalysisBasis struct {
	CaseID              string                        `json:"caseId"`
	Proposal            domain.TransmitterProposal    `json:"proposal"`
	Receivers           []domain.ProtectedReceiver    `json:"receivers"`
	Assessment          domain.InterferenceAssessment `json:"assessment"`
	InputVerified       bool                          `json:"inputVerified"`
	ComputationVerified bool                          `json:"computationVerified"`
	ComputedDigest      string                        `json:"computedDigest"`
	Verification        analysis.VerificationReport   `json:"verification"`
}

type AuthorizationVerification struct {
	AuthorizationNo    string    `json:"authorizationNo"`
	CaseID             string    `json:"caseId"`
	Valid              bool      `json:"valid"`
	FrozenDigestValid  bool      `json:"frozenDigestValid"`
	CredentialValid    bool      `json:"credentialValid"`
	ComputedDigest     string    `json:"computedDigest"`
	Reason             string    `json:"reason"`
	TimeState          string    `json:"timeState"`
	OperationallyValid bool      `json:"operationallyValid"`
	CheckedAt          time.Time `json:"checkedAt"`
	ValidFrom          time.Time `json:"validFrom"`
	ValidUntil         time.Time `json:"validUntil"`
	Conditions         []string  `json:"conditions"`
}
