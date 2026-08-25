package httpapi

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/domain"
	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/store"
)

func (a *API) GetAssessmentHandler(writer http.ResponseWriter, request *http.Request) {
	value, err := a.service.GetAnalysisBasis(request.PathValue("caseID"))
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeData(writer, http.StatusOK, value)
}

func (a *API) GetRemediationHandler(writer http.ResponseWriter, request *http.Request) {
	value, err := a.service.Remediation(request.PathValue("caseID"))
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeData(writer, http.StatusOK, value)
}
func (a *API) CompareAssessmentHandler(writer http.ResponseWriter, request *http.Request) {
	base, err := strconv.Atoi(request.URL.Query().Get("baseRevision"))
	if err != nil {
		base, err = strconv.Atoi(request.URL.Query().Get("base"))
	}
	target, e2 := strconv.Atoi(request.URL.Query().Get("targetRevision"))
	if e2 != nil {
		target, e2 = strconv.Atoi(request.URL.Query().Get("target"))
	}
	if err != nil || e2 != nil || base < 1 || target < 1 {
		writeError(writer, request, domain.ValidationError(domain.Violation("baseRevision", "positive", "分析修订号必须为正整数")))
		return
	}
	value, err := a.service.CompareAssessments(request.PathValue("caseID"), base, target)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeData(writer, http.StatusOK, value)
}

func (a *API) GetAuditHandler(writer http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	filter := store.AuditFilter{Action: query.Get("action"), Actor: query.Get("actor")}
	var err error
	if raw := query.Get("limit"); raw != "" {
		filter.Limit, err = strconv.Atoi(raw)
		if err != nil || filter.Limit < 1 || filter.Limit > 500 {
			writeError(writer, request, domain.ValidationError(domain.Violation("limit", "range", "limit 必须处于 1 至 500")))
			return
		}
	}
	if raw := query.Get("after"); raw != "" {
		filter.After, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || filter.After < 0 {
			writeError(writer, request, domain.ValidationError(domain.Violation("after", "range", "after 必须为非负序号")))
			return
		}
	}
	fromRaw := query.Get("from")
	if fromRaw == "" {
		fromRaw = query.Get("startTime")
	}
	if raw := fromRaw; raw != "" {
		t, e := time.Parse(time.RFC3339, raw)
		if e != nil {
			writeError(writer, request, domain.ValidationError(domain.Violation("from", "time", "时间必须符合 RFC3339")))
			return
		}
		filter.From = &t
	}
	toRaw := query.Get("to")
	if toRaw == "" {
		toRaw = query.Get("endTime")
	}
	if raw := toRaw; raw != "" {
		t, e := time.Parse(time.RFC3339, raw)
		if e != nil {
			writeError(writer, request, domain.ValidationError(domain.Violation("to", "time", "时间必须符合 RFC3339")))
			return
		}
		filter.To = &t
	}
	for key := range map[string]string{"minVersion": "minVersion", "maxVersion": "maxVersion"} {
		raw := query.Get(key)
		if raw == "" {
			if key == "minVersion" {
				raw = query.Get("minCaseVersion")
			} else {
				raw = query.Get("maxCaseVersion")
			}
		}
		if raw != "" {
			v, e := strconv.ParseInt(raw, 10, 64)
			if e != nil || v < 1 {
				writeError(writer, request, domain.ValidationError(domain.Violation(key, "positive", "版本范围必须为正整数")))
				return
			}
			if key == "minVersion" {
				filter.MinVersion = &v
			} else {
				filter.MaxVersion = &v
			}
		}
	}
	if filter.MinVersion != nil && filter.MaxVersion != nil && *filter.MinVersion > *filter.MaxVersion {
		writeError(writer, request, domain.ValidationError(domain.Violation("caseVersion", "range", "版本范围无效")))
		return
	}
	value, err := a.service.GetAuditFiltered(request.PathValue("caseID"), filter)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeData(writer, http.StatusOK, value)
}

func (a *API) GetReviewResponsesHandler(writer http.ResponseWriter, request *http.Request) {
	value, err := a.service.GetReviewResponseViews(request.PathValue("caseID"))
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeData(writer, http.StatusOK, value)
}

func (a *API) GetAuthorizationHandler(writer http.ResponseWriter, request *http.Request) {
	value, err := a.service.GetAuthorization(request.PathValue("caseID"))
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeData(writer, http.StatusOK, value)
}

type verifyAuthorizationRequest struct {
	CaseID          string `json:"caseId"`
	AuthorizationNo string `json:"authorizationNo"`
	At              string `json:"at,omitempty"`
}

func (a *API) VerifyAuthorizationHandler(writer http.ResponseWriter, request *http.Request) {
	var command verifyAuthorizationRequest
	if err := decodeJSON(writer, request, &command); err != nil {
		writeError(writer, request, err)
		return
	}
	if command.CaseID == "" || command.AuthorizationNo == "" {
		writeError(writer, request, domain.ValidationError(
			domain.Violation("caseId", "required", "协调案 ID 不能为空"),
			domain.Violation("authorizationNo", "required", "授权编号不能为空"),
		))
		return
	}
	at := time.Now().UTC()
	if strings.TrimSpace(command.At) != "" {
		parsed, e := time.Parse(time.RFC3339, command.At)
		if e != nil {
			writeError(writer, request, domain.ValidationError(domain.Violation("at", "time", "at 必须符合 RFC3339")))
			return
		}
		if parsed.Before(time.Unix(0, 0).UTC()) || parsed.After(time.Now().UTC().Add(100*365*24*time.Hour)) {
			writeError(writer, request, domain.ValidationError(domain.Violation("at", "range", "at 超出合理时间范围")))
			return
		}
		at = parsed
	}
	value, err := a.service.VerifyAuthorizationAt(command.CaseID, command.AuthorizationNo, at)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeData(writer, http.StatusOK, value)
}
