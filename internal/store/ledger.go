package store

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/domain"
)

const schemaVersion = 1

type ledgerEvent struct {
	SchemaVersion int                      `json:"schemaVersion"`
	Sequence      int64                    `json:"sequence"`
	CaseID        string                   `json:"caseId"`
	Action        string                   `json:"action"`
	Actor         string                   `json:"actor"`
	CaseVersion   int64                    `json:"caseVersion"`
	OccurredAt    time.Time                `json:"occurredAt"`
	Details       map[string]any           `json:"details,omitempty"`
	Case          *domain.CoordinationCase `json:"case"`
	Idempotency   *IdempotencyRecord       `json:"idempotency,omitempty"`
	PreviousHash  string                   `json:"previousHash"`
	Hash          string                   `json:"hash"`
}

type eventHashMaterial struct {
	SchemaVersion int                      `json:"schemaVersion"`
	Sequence      int64                    `json:"sequence"`
	CaseID        string                   `json:"caseId"`
	Action        string                   `json:"action"`
	Actor         string                   `json:"actor"`
	CaseVersion   int64                    `json:"caseVersion"`
	OccurredAt    time.Time                `json:"occurredAt"`
	Details       map[string]any           `json:"details,omitempty"`
	Case          *domain.CoordinationCase `json:"case"`
	Idempotency   *IdempotencyRecord       `json:"idempotency,omitempty"`
	PreviousHash  string                   `json:"previousHash"`
}

func sealEvent(event *ledgerEvent) error {
	material := eventHashMaterial{
		SchemaVersion: event.SchemaVersion, Sequence: event.Sequence, CaseID: event.CaseID,
		Action: event.Action, Actor: event.Actor, CaseVersion: event.CaseVersion,
		OccurredAt: event.OccurredAt, Details: event.Details, Case: event.Case,
		Idempotency: event.Idempotency, PreviousHash: event.PreviousHash,
	}
	payload, err := json.Marshal(material)
	if err != nil {
		return domain.WrapIntegrity("无法编码账本事件", err)
	}
	sum := sha256.Sum256(payload)
	event.Hash = hex.EncodeToString(sum[:])
	return nil
}

func verifyEvent(event ledgerEvent, expectedSequence int64, previousHash string) error {
	if event.SchemaVersion != schemaVersion {
		return domain.NewError(domain.CodeIntegrity, fmt.Sprintf("不支持的账本 schemaVersion：%d", event.SchemaVersion))
	}
	if event.Sequence != expectedSequence {
		return domain.NewError(domain.CodeIntegrity, fmt.Sprintf("账本序号不连续：期望 %d，实际 %d", expectedSequence, event.Sequence))
	}
	if event.PreviousHash != previousHash {
		return domain.NewError(domain.CodeIntegrity, fmt.Sprintf("账本第 %d 条事件前序哈希不匹配", event.Sequence))
	}
	storedHash := event.Hash
	if err := sealEvent(&event); err != nil {
		return err
	}
	if storedHash != event.Hash {
		return domain.NewError(domain.CodeIntegrity, fmt.Sprintf("账本第 %d 条事件哈希不匹配", event.Sequence))
	}
	if event.Case == nil || event.Case.ID != event.CaseID || event.Case.Version != event.CaseVersion {
		return domain.NewError(domain.CodeIntegrity, fmt.Sprintf("账本第 %d 条事件聚合投影不一致", event.Sequence))
	}
	return nil
}

func (r *Repository) appendLedger(event ledgerEvent) error {
	if r.ledgerFile == nil {
		file, err := os.OpenFile(r.ledgerPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return domain.WrapIntegrity("无法打开事件账本", err)
		}
		r.ledgerFile = file
	}
	encoder := json.NewEncoder(r.ledgerFile)
	if err := encoder.Encode(event); err != nil {
		return domain.WrapIntegrity("无法追加事件账本", err)
	}
	if err := r.ledgerFile.Sync(); err != nil {
		return domain.WrapIntegrity("无法同步事件账本", err)
	}
	return nil
}

func readLedger(path string) ([]ledgerEvent, error) {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return []ledgerEvent{}, nil
	}
	if err != nil {
		return nil, domain.WrapIntegrity("无法读取事件账本", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	events := make([]ledgerEvent, 0)
	var sequence int64 = 1
	previousHash := ""
	for scanner.Scan() {
		var event ledgerEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return nil, domain.WrapIntegrity(fmt.Sprintf("账本第 %d 行不是有效 JSON", sequence), err)
		}
		if err := verifyEvent(event, sequence, previousHash); err != nil {
			return nil, err
		}
		events = append(events, event)
		sequence++
		previousHash = event.Hash
	}
	if err := scanner.Err(); err != nil {
		return nil, domain.WrapIntegrity("扫描事件账本失败", err)
	}
	return events, nil
}
