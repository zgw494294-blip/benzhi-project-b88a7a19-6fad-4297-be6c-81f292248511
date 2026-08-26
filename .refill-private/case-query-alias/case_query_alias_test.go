package case_query_alias_test

import (
	"testing"
	"time"

	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/domain"
	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/store"
)

func TestCaseQueryDoesNotExposeRepositoryAggregate(t *testing.T) {
	repository, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC)
	value, err := domain.NewCase("case-query-alias", "查询隔离测试", "CN-44", "测试申请人", now)
	if err != nil {
		t.Fatal(err)
	}
	proposal := domain.TransmitterProposal{
		FrequencyHz: 100_000_000, BandwidthHz: 200_000, EIRPDbm: 6,
		AntennaGainDbi: 2, AntennaHeightM: 30, Latitude: 23.1, Longitude: 113.3,
		EmissionClass: "F3E", Rationale: "现场测量依据",
	}
	if err := value.ReplaceProposal(proposal, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Create(value, "case_created", "规划工程师", nil); err != nil {
		t.Fatal(err)
	}

	first, err := repository.Get(value.ID)
	if err != nil {
		t.Fatal(err)
	}
	first.Proposals[0].EIRPDbm = 99
	first.Proposals[0].Rationale = "调用方篡改"

	second, err := repository.Get(value.ID)
	if err != nil {
		t.Fatal(err)
	}
	if second.Proposals[0].EIRPDbm != 6 || second.Proposals[0].Rationale != "现场测量依据" {
		t.Fatalf("repository query was polluted through returned aggregate: %+v", second.Proposals[0])
	}
}
