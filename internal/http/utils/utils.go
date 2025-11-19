package utils

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

const (
	MaxRequestSize = 1048576 // 1mb
)

func DecodeJSON(w http.ResponseWriter, r *http.Request, v interface{}) error {
	if r.Body == nil {
		WriteResponse(w, http.StatusBadRequest, BadRequest(ErrRequestBodyEmpty))
		return fmt.Errorf("request body is empty")
	}

	r.Body = http.MaxBytesReader(w, r.Body, MaxRequestSize)
	defer func() {
		if err := r.Body.Close(); err != nil {
			log.Printf("failed request body close: %v", err)
		}
	}()

	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		log.Printf("failed to decode JSON: %v", err)
		WriteResponse(w, http.StatusBadRequest, BadRequest(ErrJSONEncoding))
		return fmt.Errorf("invalid input data")
	}

	return nil
}

func WriteResponse(w http.ResponseWriter, statusCode int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encoding failed: %v", err)
	}
}
