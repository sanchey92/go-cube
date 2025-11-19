package utils

import (
	"errors"
	"net/http"
)

var (
	ErrRequestBodyEmpty = errors.New("request body is empty")
	ErrJSONEncoding     = errors.New("failed json encoding")
	ErrInvalidTaskID    = errors.New("taskID is empty")
	ErrTaskNotFound     = errors.New("task not found")
	ErrInvalidTaskState = errors.New("task is not in a valid state for this operation")
)

type ErrResponse struct {
	HTTPStatusCode int
	Message        string
}

func BadRequest(err error) *ErrResponse {
	return newErrResponse(http.StatusBadRequest, err)
}

func InternalServerErr(err error) *ErrResponse {
	return newErrResponse(http.StatusInternalServerError, err)
}

func NotFound(err error) *ErrResponse {
	return newErrResponse(http.StatusNotFound, err)
}

func Conflict(err error) *ErrResponse {
	return newErrResponse(http.StatusConflict, err)
}

func newErrResponse(statusCode int, err error) *ErrResponse {
	return &ErrResponse{
		HTTPStatusCode: statusCode,
		Message:        err.Error(),
	}
}
