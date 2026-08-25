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
