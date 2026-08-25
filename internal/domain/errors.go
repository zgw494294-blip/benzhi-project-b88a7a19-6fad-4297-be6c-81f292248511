package domain

import (
	"errors"
	"fmt"
)

const (
	CodeValidation        = "validation_error"
	CodeNotFound          = "not_found"
	CodeVersionConflict   = "version_conflict"
	CodeStateConflict     = "state_conflict"
	CodeIdempotency       = "idempotency_conflict"
	CodeIntegrity         = "integrity_error"
	CodeAlreadyExists     = "already_exists"
	CodeUnauthorizedActor = "unauthorized_actor"
)

type FieldViolation struct {
	Field   string `json:"field"`
	Rule    string `json:"rule"`
	Message string `json:"message"`
}

type Error struct {
	Code       string           `json:"code"`
	Message    string           `json:"message"`
	Violations []FieldViolation `json:"violations,omitempty"`
	Cause      error            `json:"-"`
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *Error) Unwrap() error { return e.Cause }

func NewError(code, message string) *Error {
	return &Error{Code: code, Message: message}
}

func ValidationError(violations ...FieldViolation) *Error {
	return &Error{Code: CodeValidation, Message: "请求参数未通过校验", Violations: violations}
}

func Violation(field, rule, message string) FieldViolation {
	return FieldViolation{Field: field, Rule: rule, Message: message}
}

func IsCode(err error, code string) bool {
	var target *Error
	return errors.As(err, &target) && target.Code == code
}

func WrapIntegrity(message string, cause error) *Error {
	return &Error{Code: CodeIntegrity, Message: message, Cause: cause}
}
