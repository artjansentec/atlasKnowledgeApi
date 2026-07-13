package httperr

import (
	"errors"
	"fmt"
	"net/http"
)

type Error struct {
	Code       string
	Message    string
	StatusCode int
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func New(status int, code, message string) *Error {
	return &Error{StatusCode: status, Code: code, Message: message}
}

func Unauthorized(message string) *Error {
	return New(http.StatusUnauthorized, "UNAUTHORIZED", message)
}

func Forbidden(message string) *Error {
	return New(http.StatusForbidden, "FORBIDDEN", message)
}

func NotFound(message string) *Error {
	return New(http.StatusNotFound, "NOT_FOUND", message)
}

func BadRequest(message string) *Error {
	return New(http.StatusBadRequest, "BAD_REQUEST", message)
}

func Validation(message string) *Error {
	return New(http.StatusUnprocessableEntity, "VALIDATION_ERROR", message)
}

func InvalidStatus(message string) *Error {
	return New(http.StatusBadRequest, "INVALID_STATUS", message)
}

func Internal(message string) *Error {
	return New(http.StatusInternalServerError, "INTERNAL_ERROR", message)
}

func Conflict(message string) *Error {
	return New(http.StatusConflict, "CONFLICT", message)
}

func PayloadTooLarge(message string) *Error {
	return New(http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", message)
}

func UnsupportedMediaType(message string) *Error {
	return New(http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", message)
}

func BadGateway(message string) *Error {
	return New(http.StatusBadGateway, "BAD_GATEWAY", message)
}

func GatewayTimeout(message string) *Error {
	return New(http.StatusGatewayTimeout, "GATEWAY_TIMEOUT", message)
}

func AsHTTPError(err error) *Error {
	var httpErr *Error
	if errors.As(err, &httpErr) {
		return httpErr
	}
	return Internal("erro interno do servidor")
}
