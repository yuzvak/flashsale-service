package response

import (
	"encoding/json"
	"net/http"
)

type Status string

const (
	StatusSuccess            Status = "success"
	StatusError              Status = "error"
	StatusValidationError    Status = "validation_error"
	StatusNotFound           Status = "not_found"
	StatusUnauthorized       Status = "unauthorized"
	StatusForbidden          Status = "forbidden"
	StatusConflict           Status = "conflict"
	StatusInternalError      Status = "internal_error"
	StatusServiceUnavailable Status = "service_unavailable"
)

type BaseResponse struct {
	Message string `json:"message,omitempty"`
}

type DataResponse[T any] struct {
	BaseResponse
	Data T `json:"data,omitempty"`
}

type ErrorResponse struct {
	BaseResponse
	Error string `json:"error,omitempty"`
	Code  string `json:"code,omitempty"`
}

type ValidationErrorResponse struct {
	BaseResponse
	Errors map[string]string `json:"errors,omitempty"`
}

func Success[T any](data T, message ...string) *DataResponse[T] {
	return &DataResponse[T]{
		Data: data,
	}
}

func Error(status Status, message string, errorDetails ...string) *ErrorResponse {
	return &ErrorResponse{
		BaseResponse: BaseResponse{
			Message: message,
		},
	}
}

func ValidationError(message string, errors map[string]string) *ValidationErrorResponse {
	return &ValidationErrorResponse{
		BaseResponse: BaseResponse{
			Message: message,
		},
	}
}

func WriteJSON(w http.ResponseWriter, statusCode int, response interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

func WriteSuccess[T any](w http.ResponseWriter, data T, message ...string) {
	WriteJSON(w, http.StatusOK, data)
}

func WriteError(w http.ResponseWriter, statusCode int, status Status, message string, errorDetails ...string) {
	WriteJSON(w, statusCode, Error(status, message, errorDetails...))
}

func WriteValidationError(w http.ResponseWriter, message string, errors map[string]string) {
	WriteJSON(w, http.StatusBadRequest, ValidationError(message, errors))
}
