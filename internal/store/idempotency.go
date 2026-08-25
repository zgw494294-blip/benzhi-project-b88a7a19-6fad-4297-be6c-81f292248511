package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/domain"
)

type IdempotencyRequest struct {
	Key         string
	Fingerprint string
}

type IdempotencyRecord struct {
	Key          string                   `json:"key"`
	Fingerprint  string                   `json:"fingerprint"`
	CaseID       string                   `json:"caseId"`
	Action       string                   `json:"action"`
	CaseSnapshot *domain.CoordinationCase `json:"caseSnapshot"`
	RecordedAt   time.Time                `json:"recordedAt"`
}

func Fingerprint(value any) (string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "", domain.WrapIntegrity("无法生成请求指纹", err)
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func validateIdempotency(request *IdempotencyRequest) error {
	if request == nil {
		return nil
	}
	if strings.TrimSpace(request.Key) == "" {
		return domain.ValidationError(domain.Violation("idempotencyKey", "required", "idempotencyKey 不能为空"))
	}
	if len(request.Key) > 128 {
		return domain.ValidationError(domain.Violation("idempotencyKey", "max_length", "idempotencyKey 不能超过 128 个字符"))
	}
	if request.Fingerprint == "" {
		return domain.NewError(domain.CodeIntegrity, "幂等请求缺少指纹")
	}
	return nil
}

func idempotencyIndexKey(caseID, action, key string) string {
	return caseID + "\x00" + action + "\x00" + key
}
