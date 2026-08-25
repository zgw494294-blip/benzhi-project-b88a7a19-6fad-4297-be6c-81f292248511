package httpapi

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/analysis"
	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/coordination"
	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/store"
)

func TestRunSelfcheckAgainstHTTPServer(t *testing.T) {
	repository, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(New(coordination.NewService(repository, analysis.NewEngine())))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := RunSelfcheck(ctx, server.URL); err != nil {
		t.Fatalf("RunSelfcheck() error = %v", err)
	}
}
