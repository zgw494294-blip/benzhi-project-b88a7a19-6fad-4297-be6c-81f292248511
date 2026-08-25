package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"benzhi-project-b88a7a19-6fad-4297-be6c-81f292248511/internal/domain"
)

type contextKey string

const requestIDKey contextKey = "requestID"

type responseEnvelope struct {
	Data any            `json:"data"`
	Meta map[string]any `json:"meta,omitempty"`
}

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code       string                  `json:"code"`
	Message    string                  `json:"message"`
	Violations []domain.FieldViolation `json:"violations,omitempty"`
	RequestID  string                  `json:"requestId"`
}

func withRequestID(ctx context.Context, value string) context.Context {
	return context.WithValue(ctx, requestIDKey, value)
}

func requestID(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey).(string)
	return value
}

func decodeJSON(writer http.ResponseWriter, request *http.Request, target any) error {
	if contentType := request.Header.Get("Content-Type"); contentType != "" && !strings.HasPrefix(strings.ToLower(contentType), "application/json") {
		return domain.ValidationError(domain.Violation("Content-Type", "media_type", "请求必须使用 application/json"))
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return domain.ValidationError(domain.Violation("body", "json", jsonDecodeMessage(err)))
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return domain.ValidationError(domain.Violation("body", "single_document", "请求体只能包含一个 JSON 对象"))
	}
	return nil
}

func jsonDecodeMessage(err error) string {
	var syntax *json.SyntaxError
	var typeError *json.UnmarshalTypeError
	switch {
	case errors.As(err, &syntax):
		return "JSON 语法错误"
	case errors.As(err, &typeError):
		return "字段 " + typeError.Field + " 的值类型不正确"
	case errors.Is(err, io.EOF):
		return "请求体不能为空"
	case strings.HasPrefix(err.Error(), "json: unknown field "):
		return "请求包含未知字段 " + strings.TrimPrefix(err.Error(), "json: unknown field ")
	case strings.Contains(err.Error(), "request body too large"):
		return "请求体不能超过 1 MiB"
	default:
		return "请求体不是有效 JSON"
	}
}

func writeData(writer http.ResponseWriter, status int, data any) {
	writeEnvelope(writer, status, responseEnvelope{Data: data})
}

func writeDataWithMeta(writer http.ResponseWriter, status int, data any, meta map[string]any) {
	writeEnvelope(writer, status, responseEnvelope{Data: data, Meta: meta})
}

func writeEnvelope(writer http.ResponseWriter, status int, payload any) {
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(payload)
}

func writeError(writer http.ResponseWriter, request *http.Request, err error) {
	status := http.StatusInternalServerError
	body := errorBody{Code: "internal_error", Message: "服务内部错误", RequestID: requestID(request.Context())}
	var typed *domain.Error
	if errors.As(err, &typed) {
		body.Code = typed.Code
		body.Message = typed.Message
		body.Violations = typed.Violations
		switch typed.Code {
		case domain.CodeValidation:
			status = http.StatusBadRequest
		case domain.CodeNotFound:
			status = http.StatusNotFound
		case domain.CodeVersionConflict, domain.CodeStateConflict, domain.CodeIdempotency, domain.CodeAlreadyExists:
			status = http.StatusConflict
		case domain.CodeUnauthorizedActor:
			status = http.StatusForbidden
		case domain.CodeIntegrity:
			status = http.StatusInternalServerError
		}
	}
	writeEnvelope(writer, status, errorEnvelope{Error: body})
}
