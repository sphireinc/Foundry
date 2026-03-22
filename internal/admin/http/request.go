package httpadmin

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

const (
	smallJSONBodyLimit    int64 = 64 << 10
	mediumJSONBodyLimit   int64 = 1 << 20
	largeJSONBodyLimit    int64 = 8 << 20
	configJSONBodyLimit   int64 = 2 << 20
	adminMultipartMaxBody int64 = adminMediaUploadLimit + (1 << 20)
)

type requestBodyError struct {
	status  int
	message string
}

func (e *requestBodyError) Error() string {
	return e.message
}

func decodeJSONBody(w http.ResponseWriter, req *http.Request, limit int64, dst any) error {
	if req == nil || req.Body == nil {
		return &requestBodyError{status: http.StatusBadRequest, message: "request body is required"}
	}
	if limit > 0 {
		req.Body = http.MaxBytesReader(w, req.Body, limit)
	}
	dec := json.NewDecoder(req.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		switch {
		case errors.As(err, &maxErr):
			return &requestBodyError{status: http.StatusRequestEntityTooLarge, message: fmt.Sprintf("request body exceeds %d bytes", limit)}
		case errors.Is(err, io.EOF):
			return &requestBodyError{status: http.StatusBadRequest, message: "request body is required"}
		default:
			return &requestBodyError{status: http.StatusBadRequest, message: "invalid JSON request body"}
		}
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return &requestBodyError{status: http.StatusBadRequest, message: "request body must contain a single JSON object"}
		}
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return &requestBodyError{status: http.StatusRequestEntityTooLarge, message: fmt.Sprintf("request body exceeds %d bytes", limit)}
		}
		return &requestBodyError{status: http.StatusBadRequest, message: "invalid JSON request body"}
	}
	return nil
}

func writeRequestBodyError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	var bodyErr *requestBodyError
	if !errors.As(err, &bodyErr) {
		return false
	}
	writeJSONErrorMessage(w, bodyErr.status, bodyErr.message)
	return true
}
