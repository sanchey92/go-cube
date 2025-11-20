package utils

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/sanchey92/go-cube/pkg/errors"
)

const (
	MaxRequestSize = 1048576 // 1mb
)

func DecodeJSON(w http.ResponseWriter, r *http.Request, v interface{}) error {
	if r.Body == nil {
		WriteResponse(w, http.StatusBadRequest, errors.BadRequest(errors.ErrRequestBodyEmpty))
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
		WriteResponse(w, http.StatusBadRequest, errors.BadRequest(errors.ErrJSONEncoding))
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

func HTTPWithTRetry(f func(string) (*http.Response, error), url string) (*http.Response, error) {
	count := 10
	var resp *http.Response
	var err error

	for i := 0; i < count; i++ {
		resp, err = f(url)
		if err != nil {
			fmt.Printf("Err callong url: %v\n", url)
			time.Sleep(time.Second * 5)
		} else {
			break
		}
	}
	return resp, err
}
