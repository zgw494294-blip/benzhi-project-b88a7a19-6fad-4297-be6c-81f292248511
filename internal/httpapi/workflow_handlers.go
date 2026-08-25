package httpapi

import (
	"net/http"
	"strconv"

	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/coordination"
	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/domain"
)

func (a *API) RunAnalysisHandler(writer http.ResponseWriter, request *http.Request) {
	var command coordination.VersionedCommand
	if err := decodeJSON(writer, request, &command); err != nil {
		writeError(writer, request, err)
		return
	}
	value, err := a.service.RunAnalysisContext(request.Context(), request.PathValue("caseID"), command)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeData(writer, http.StatusCreated, value)
}

func (a *API) SubmitForReviewHandler(writer http.ResponseWriter, request *http.Request) {
	var command coordination.VersionedCommand
	if err := decodeJSON(writer, request, &command); err != nil {
		writeError(writer, request, err)
		return
	}
	value, err := a.service.SubmitForReview(request.PathValue("caseID"), command)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeData(writer, http.StatusOK, value)
}

func (a *API) DecideReviewHandler(writer http.ResponseWriter, request *http.Request) {
	var command coordination.ReviewCommand
	if err := decodeJSON(writer, request, &command); err != nil {
		writeError(writer, request, err)
		return
	}
	value, err := a.service.DecideReview(request.PathValue("caseID"), command)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeData(writer, http.StatusCreated, value)
}

func (a *API) AddReviewResponseHandler(writer http.ResponseWriter, request *http.Request) {
	var command coordination.ReviewResponseCommand
	if err := decodeJSON(writer, request, &command); err != nil {
		writeError(writer, request, err)
		return
	}
	value, err := a.service.AddReviewResponse(request.PathValue("caseID"), command)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeData(writer, http.StatusOK, value)
}

func (a *API) FreezeHandler(writer http.ResponseWriter, request *http.Request) {
	var command coordination.FreezeCommand
	if err := decodeJSON(writer, request, &command); err != nil {
		writeError(writer, request, err)
		return
	}
	value, err := a.service.Freeze(request.PathValue("caseID"), command)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeData(writer, http.StatusOK, value)
}

func (a *API) FreezePreflightHandler(writer http.ResponseWriter, request *http.Request) {
	var command coordination.VersionedCommand
	if err := decodeJSON(writer, request, &command); err != nil {
		writeError(writer, request, err)
		return
	}
	value, err := a.service.FreezePreflight(request.PathValue("caseID"), command.ExpectedVersion)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeData(writer, http.StatusOK, value)
}
func (a *API) FreezePreflightQueryHandler(writer http.ResponseWriter, request *http.Request) {
	version, err := strconv.ParseInt(request.URL.Query().Get("expectedVersion"), 10, 64)
	if err != nil {
		writeError(writer, request, domain.ValidationError(domain.Violation("expectedVersion", "positive", "expectedVersion 必须为正整数")))
		return
	}
	value, err := a.service.FreezePreflight(request.PathValue("caseID"), version)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeData(writer, http.StatusOK, value)
}

func (a *API) IssueAuthorizationHandler(writer http.ResponseWriter, request *http.Request) {
	var command coordination.AuthorizationCommand
	if err := decodeJSON(writer, request, &command); err != nil {
		writeError(writer, request, err)
		return
	}
	value, err := a.service.IssueAuthorization(request.PathValue("caseID"), command)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeData(writer, http.StatusCreated, value.Authorization)
}
