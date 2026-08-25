package coordination

import (
	"sort"
	"strings"

	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/domain"
	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/store"
)

func (s *Service) CreateCase(command CreateCaseCommand) (*domain.CoordinationCase, error) {
	if err := requireActor(command.Actor); err != nil {
		return nil, err
	}
	value, err := domain.NewCase(s.newID("case"), command.Title, command.RegionCode, command.Applicant, s.clock())
	if err != nil {
		return nil, err
	}
	return s.repository.Create(value, "case_created", strings.TrimSpace(command.Actor), map[string]any{"title": value.Title, "regionCode": value.RegionCode})
}

func (s *Service) ReplaceProposal(caseID string, command ProposalCommand) (*domain.CoordinationCase, error) {
	if err := requireActor(command.Actor); err != nil {
		return nil, err
	}
	proposal := domain.TransmitterProposal{
		FrequencyHz: command.FrequencyHz, BandwidthHz: command.BandwidthHz, EIRPDbm: command.EIRPDbm,
		AntennaGainDbi: command.AntennaGainDbi, AntennaHeightM: command.AntennaHeightM,
		Latitude: command.Latitude, Longitude: command.Longitude, EmissionClass: command.EmissionClass, Rationale: command.Rationale,
	}
	value, _, err := s.repository.Update(caseID, command.ExpectedVersion, "proposal_revised", strings.TrimSpace(command.Actor), map[string]any{"frequencyHz": command.FrequencyHz}, nil, func(value *domain.CoordinationCase) error {
		return value.ReplaceProposal(proposal, s.clock())
	})
	return value, err
}

func (s *Service) AddReceiver(caseID string, command ReceiverCommand) (*domain.CoordinationCase, error) {
	if err := requireActor(command.Actor); err != nil {
		return nil, err
	}
	receiver := domain.ProtectedReceiver{
		ID: command.ID, Label: command.Label, Latitude: command.Latitude, Longitude: command.Longitude,
		ReceiveFrequencyHz: command.ReceiveFrequencyHz, ProtectionThresholdDbm: command.ProtectionThresholdDbm,
		AntennaGainDbi: command.AntennaGainDbi, TerrainClass: command.TerrainClass, EvidenceRef: command.EvidenceRef,
	}
	value, _, err := s.repository.Update(caseID, command.ExpectedVersion, "receiver_added", strings.TrimSpace(command.Actor), map[string]any{"receiverId": command.ID, "label": command.Label}, nil, func(value *domain.CoordinationCase) error {
		return value.AddReceiver(receiver, s.clock())
	})
	return value, err
}

func (s *Service) AddReceiversBatch(caseID string, command BatchReceiverCommand) (*domain.CoordinationCase, error) {
	if err := requireActor(command.Actor); err != nil {
		return nil, err
	}
	receivers := make([]domain.ProtectedReceiver, len(command.Receivers))
	for i, item := range command.Receivers {
		receivers[i] = domain.ProtectedReceiver{ID: item.ID, Label: item.Label, Latitude: item.Latitude, Longitude: item.Longitude, ReceiveFrequencyHz: item.ReceiveFrequencyHz, ProtectionThresholdDbm: item.ProtectionThresholdDbm, AntennaGainDbi: item.AntennaGainDbi, TerrainClass: item.TerrainClass, EvidenceRef: item.EvidenceRef}
	}
	value, _, err := s.repository.Update(caseID, command.ExpectedVersion, "receivers_batch_added", strings.TrimSpace(command.Actor), map[string]any{"receiverCount": len(receivers), "receiverIds": receiverIDs(receivers)}, nil, func(value *domain.CoordinationCase) error { return value.AddReceivers(receivers, s.clock()) })
	return value, err
}

func receiverIDs(receivers []domain.ProtectedReceiver) []string {
	ids := make([]string, len(receivers))
	for i, r := range receivers {
		ids[i] = r.ID
	}
	sort.Strings(ids)
	return ids
}

func (s *Service) ReplaceReceiver(caseID, receiverID string, command ReceiverCommand) (*domain.CoordinationCase, error) {
	if err := requireActor(command.Actor); err != nil {
		return nil, err
	}
	receiver := domain.ProtectedReceiver{
		Label: command.Label, Latitude: command.Latitude, Longitude: command.Longitude,
		ReceiveFrequencyHz: command.ReceiveFrequencyHz, ProtectionThresholdDbm: command.ProtectionThresholdDbm,
		AntennaGainDbi: command.AntennaGainDbi, TerrainClass: command.TerrainClass, EvidenceRef: command.EvidenceRef,
	}
	value, _, err := s.repository.Update(caseID, command.ExpectedVersion, "receiver_revised", strings.TrimSpace(command.Actor), map[string]any{"receiverId": receiverID, "label": command.Label}, nil, func(value *domain.CoordinationCase) error {
		return value.ReplaceReceiver(receiverID, receiver, s.clock())
	})
	return value, err
}

func (s *Service) SubmitForAnalysis(caseID string, command SubmitCommand) (*domain.CoordinationCase, bool, error) {
	if err := requireActor(command.Actor); err != nil {
		return nil, false, err
	}
	fingerprint, err := store.Fingerprint(struct {
		CaseID          string `json:"caseId"`
		ExpectedVersion int64  `json:"expectedVersion"`
		Actor           string `json:"actor"`
	}{CaseID: caseID, ExpectedVersion: command.ExpectedVersion, Actor: strings.TrimSpace(command.Actor)})
	if err != nil {
		return nil, false, err
	}
	idem := &store.IdempotencyRequest{Key: command.IdempotencyKey, Fingerprint: fingerprint}
	return s.repository.Update(caseID, command.ExpectedVersion, "analysis_submitted", strings.TrimSpace(command.Actor), map[string]any{"idempotencyKey": command.IdempotencyKey}, idem, func(value *domain.CoordinationCase) error {
		return value.SubmitForAnalysis(s.clock())
	})
}
