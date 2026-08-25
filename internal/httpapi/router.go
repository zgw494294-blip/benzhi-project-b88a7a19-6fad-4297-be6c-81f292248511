package httpapi

import (
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/coordination"
)

type API struct {
	service  *coordination.Service
	requests atomic.Uint64
}

func New(service *coordination.Service) http.Handler {
	api := &API{service: service}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", api.HealthHandler)
	mux.HandleFunc("POST /api/v1/cases", api.CreateCaseHandler)
	mux.HandleFunc("GET /api/v1/cases/{caseID}", api.GetCaseHandler)
	mux.HandleFunc("PUT /api/v1/cases/{caseID}/proposal", api.ReplaceProposalHandler)
	mux.HandleFunc("POST /api/v1/cases/{caseID}/receivers", api.AddReceiverHandler)
	mux.HandleFunc("POST /api/v1/cases/{caseID}/receivers/batch", api.AddReceiversBatchHandler)
	mux.HandleFunc("PUT /api/v1/cases/{caseID}/receivers/{receiverID}", api.ReplaceReceiverHandler)
	mux.HandleFunc("POST /api/v1/cases/{caseID}/submit", api.SubmitForAnalysisHandler)
	mux.HandleFunc("POST /api/v1/cases/{caseID}/submit/preflight", api.PreflightHandler)
	mux.HandleFunc("POST /api/v1/cases/{caseID}/preflight", api.PreflightHandler)
	mux.HandleFunc("GET /api/v1/cases/{caseID}/preflight", api.PreflightQueryHandler)
	mux.HandleFunc("POST /api/v1/cases/{caseID}/assessments", api.RunAnalysisHandler)
	mux.HandleFunc("GET /api/v1/cases/{caseID}/assessment", api.GetAssessmentHandler)
	mux.HandleFunc("GET /api/v1/cases/{caseID}/assessment/remediation", api.GetRemediationHandler)
	mux.HandleFunc("GET /api/v1/cases/{caseID}/assessment/compare", api.CompareAssessmentHandler)
	mux.HandleFunc("POST /api/v1/cases/{caseID}/review-submissions", api.SubmitForReviewHandler)
	mux.HandleFunc("POST /api/v1/cases/{caseID}/reviews", api.DecideReviewHandler)
	mux.HandleFunc("POST /api/v1/cases/{caseID}/review-responses", api.AddReviewResponseHandler)
	mux.HandleFunc("GET /api/v1/cases/{caseID}/review-responses", api.GetReviewResponsesHandler)
	mux.HandleFunc("POST /api/v1/cases/{caseID}/freeze", api.FreezeHandler)
	mux.HandleFunc("POST /api/v1/cases/{caseID}/freeze/preflight", api.FreezePreflightHandler)
	mux.HandleFunc("GET /api/v1/cases/{caseID}/freeze/preflight", api.FreezePreflightQueryHandler)
	mux.HandleFunc("POST /api/v1/cases/{caseID}/authorizations", api.IssueAuthorizationHandler)
	mux.HandleFunc("GET /api/v1/cases/{caseID}/authorization", api.GetAuthorizationHandler)
	mux.HandleFunc("GET /api/v1/cases/{caseID}/audit", api.GetAuditHandler)
	mux.HandleFunc("POST /api/v1/authorizations/verify", api.VerifyAuthorizationHandler)
	return api.middleware(mux)
}

func (a *API) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestID := "req-" + strconv.FormatUint(a.requests.Add(1), 10)
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		writer.Header().Set("X-Request-ID", requestID)
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		started := time.Now()
		next.ServeHTTP(writer, request.WithContext(withRequestID(request.Context(), requestID)))
		_ = started
	})
}

func (a *API) HealthHandler(writer http.ResponseWriter, request *http.Request) {
	writeData(writer, http.StatusOK, map[string]any{"status": "ok", "time": time.Now().UTC()})
}
