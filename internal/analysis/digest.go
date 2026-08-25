package analysis

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/domain"
)

func inputDigest(version, caseID string, proposal domain.TransmitterProposal, receivers []domain.ProtectedReceiver) (string, error) {
	normalizedReceivers := append([]domain.ProtectedReceiver(nil), receivers...)
	sort.Slice(normalizedReceivers, func(i, j int) bool {
		return normalizedReceivers[i].ID < normalizedReceivers[j].ID
	})
	document := normalizedInput{AlgorithmVersion: version, CaseID: caseID, Proposal: proposal, Receivers: normalizedReceivers}
	payload, err := json.Marshal(document)
	if err != nil {
		return "", domain.WrapIntegrity("无法编码分析输入", err)
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}
