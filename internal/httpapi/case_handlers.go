package httpapi

import (
	"net/http"
	"strconv"

	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/coordination"
	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/domain"
)

func (a *API) CreateCaseHandler(writer http.ResponseWriter, request *http.Request) {
	var command coordination.CreateCaseCommand
	if err := decodeJSON(writer, request, &command); err != nil {
		writeError(writer, request, err)
		return
	}
	value, err := a.service.CreateCase(command)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeData(writer, http.StatusCreated, value)
}

func (a *API) GetCaseHandler(writer http.ResponseWriter, request *http.Request) {
	value, err := a.service.GetCase(request.PathValue("caseID"))
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeData(writer, http.StatusOK, value)
}

func (a *API) ReplaceProposalHandler(writer http.ResponseWriter, request *http.Request) {
	var command coordination.ProposalCommand
	if err := decodeJSON(writer, request, &command); err != nil {
		writeError(writer, request, err)
		return
	}
	value, err := a.service.ReplaceProposal(request.PathValue("caseID"), command)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeData(writer, http.StatusOK, value)
}

func (a *API) AddReceiverHandler(writer http.ResponseWriter, request *http.Request) {
	var command coordination.ReceiverCommand
	if err := decodeJSON(writer, request, &command); err != nil {
		writeError(writer, request, err)
		return
	}
	value, err := a.service.AddReceiver(request.PathValue("caseID"), command)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeData(writer, http.StatusCreated, value)
}

func (a *API) AddReceiversBatchHandler(writer http.ResponseWriter, request *http.Request) {
	var command coordination.BatchReceiverCommand
	if err := decodeJSON(writer, request, &command); err != nil {
		writeError(writer, request, err)
		return
	}
	value, err := a.service.AddReceiversBatch(request.PathValue("caseID"), command)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeData(writer, http.StatusCreated, value)
}

func (a *API) ReplaceReceiverHandler(writer http.ResponseWriter, request *http.Request) {
	var command coordination.ReceiverCommand
	if err := decodeJSON(writer, request, &command); err != nil {
		writeError(writer, request, err)
		return
	}
	value, err := a.service.ReplaceReceiver(request.PathValue("caseID"), request.PathValue("receiverID"), command)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeData(writer, http.StatusOK, value)
}

func (a *API) SubmitForAnalysisHandler(writer http.ResponseWriter, request *http.Request) {
	var command coordination.SubmitCommand
	if err := decodeJSON(writer, request, &command); err != nil {
		writeError(writer, request, err)
		return
	}
	value, replay, err := a.service.SubmitForAnalysis(request.PathValue("caseID"), command)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeDataWithMeta(writer, http.StatusOK, value, map[string]any{"idempotentReplay": replay})
}

func (a *API) PreflightHandler(writer http.ResponseWriter, request *http.Request) {
	var command coordination.VersionedCommand
	if err := decodeJSON(writer, request, &command); err != nil {
		writeError(writer, request, err)
		return
	}
	value, err := a.service.Preflight(request.PathValue("caseID"), command.ExpectedVersion)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeData(writer, http.StatusOK, value)
}
func (a *API) PreflightQueryHandler(writer http.ResponseWriter, request *http.Request) {
	version, err := strconv.ParseInt(request.URL.Query().Get("expectedVersion"), 10, 64)
	if err != nil {
		writeError(writer, request, domain.ValidationError(domain.Violation("expectedVersion", "positive", "expectedVersion 必须为正整数")))
		return
	}
	value, err := a.service.Preflight(request.PathValue("caseID"), version)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeData(writer, http.StatusOK, value)
}
