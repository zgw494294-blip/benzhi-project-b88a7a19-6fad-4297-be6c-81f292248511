package coordination

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/domain"
)

type Service struct {
	repository Repository
	engine     AssessmentEngine
	clock      Clock
	newID      IDGenerator
}

func NewService(repository Repository, engine AssessmentEngine) *Service {
	return &Service{repository: repository, engine: engine, clock: func() time.Time { return time.Now().UTC() }, newID: randomID}
}

func NewServiceWithDependencies(repository Repository, engine AssessmentEngine, clock Clock, newID IDGenerator) *Service {
	return &Service{repository: repository, engine: engine, clock: clock, newID: newID}
}

func randomID(prefix string) string {
	buffer := make([]byte, 10)
	if _, err := rand.Read(buffer); err != nil {
		return prefix + "-" + time.Now().UTC().Format("20060102150405.000000000")
	}
	return prefix + "-" + hex.EncodeToString(buffer)
}

func requireActor(actor string) error {
	if strings.TrimSpace(actor) == "" {
		return domain.ValidationError(domain.Violation("actor", "required", "操作人不能为空"))
	}
	return nil
}

// requireEngine guards against a service constructed without an analysis engine.
// When the engine dependency is absent (a nil AssessmentEngine), invoking any of
// its methods would dereference a nil interface and panic. Callers cannot recover
// such failures, so the query surfaces a domain.CodeIntegrity error that
// errors.As can match, without touching the coordination case or audit trail.
func requireEngine(engine AssessmentEngine) error {
	if engine == nil {
		return domain.NewError(domain.CodeIntegrity, "协调服务缺少可用的分析引擎")
	}
	return nil
}
