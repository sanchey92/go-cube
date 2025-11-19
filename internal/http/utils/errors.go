package utils

import (
	"errors"
	"net/http"
)

var (
	ErrRequestBodyEmpty = errors.New("request body is empty")
	ErrJSONEncoding     = errors.New("failed json encoding")
)

type ErrResponse struct {
	HTTPStatusCode int
	Message        string
}

func BadRequest(err error) *ErrResponse {
	return newErrResponse(http.StatusBadRequest, err)
}

func newErrResponse(statusCode int, err error) *ErrResponse {
	return &ErrResponse{
		HTTPStatusCode: statusCode,
		Message:        err.Error(),
	}
}
