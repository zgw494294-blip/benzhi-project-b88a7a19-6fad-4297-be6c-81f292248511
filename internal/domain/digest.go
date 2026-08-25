package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"time"
)

type frozenDigestDocument struct {
	CaseID     string                 `json:"caseId"`
	Title      string                 `json:"title"`
	RegionCode string                 `json:"regionCode"`
	Applicant  string                 `json:"applicant"`
	Proposal   TransmitterProposal    `json:"proposal"`
	Receivers  []ProtectedReceiver    `json:"receivers"`
	Assessment InterferenceAssessment `json:"assessment"`
	Review     ReviewDecision         `json:"review"`
}

type authorizationDigestDocument struct {
	AuthorizationNo string   `json:"authorizationNo"`
	CaseID          string   `json:"caseId"`
	FrozenDigest    string   `json:"frozenDigest"`
	ValidFrom       string   `json:"validFrom"`
	ValidUntil      string   `json:"validUntil"`
	Conditions      []string `json:"conditions"`
	Issuer          string   `json:"issuer"`
	IssuedAt        string   `json:"issuedAt"`
}

func FrozenDigest(c *CoordinationCase, review ReviewDecision) (string, error) {
	proposal, ok := c.LatestProposal()
	if !ok {
		return "", NewError(CodeIntegrity, "无法找到冻结候选修订")
	}
	assessment, ok := c.LatestAssessment()
	if !ok {
		return "", NewError(CodeIntegrity, "无法找到冻结分析修订")
	}
	receivers := append([]ProtectedReceiver(nil), c.Receivers...)
	sort.Slice(receivers, func(i, j int) bool { return receivers[i].ID < receivers[j].ID })
	document := frozenDigestDocument{CaseID: c.ID, Title: c.Title, RegionCode: c.RegionCode, Applicant: c.Applicant, Proposal: proposal, Receivers: receivers, Assessment: assessment, Review: review}
	return digestJSON(document)
}

func AuthorizationDigest(a TrialAuthorization) (string, error) {
	conditions := append([]string(nil), a.Conditions...)
	document := authorizationDigestDocument{
		AuthorizationNo: a.AuthorizationNo, CaseID: a.CaseID, FrozenDigest: a.FrozenDigest,
		ValidFrom: canonicalTime(a.ValidFrom), ValidUntil: canonicalTime(a.ValidUntil),
		Conditions: conditions, Issuer: a.Issuer, IssuedAt: canonicalTime(a.IssuedAt),
	}
	return digestJSON(document)
}

func VerifyAuthorization(a TrialAuthorization) (bool, string, error) {
	stored := a.VerificationDigest
	computed, err := AuthorizationDigest(a)
	if err != nil {
		return false, "", err
	}
	return stored == computed, computed, nil
}

func digestJSON(value any) (string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "", WrapIntegrity("无法编码摘要内容", err)
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func canonicalTime(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }
