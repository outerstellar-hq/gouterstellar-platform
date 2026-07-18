package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// ErrMultipleJSONValues means a request body contained more than one JSON
// value. DecodeJSON requires one complete value so ignored trailing commands
// cannot diverge between middleware and handlers.
var ErrMultipleJSONValues = errors.New("request body must contain exactly one JSON value")

// DecodeJSON decodes one bounded JSON request body into destination. It
// rejects unknown object fields, malformed trailing data, and additional JSON
// values. maxBytes includes trailing whitespace and must be positive.
//
// Size failures wrap *http.MaxBytesError, allowing callers to distinguish a
// body that is too large with errors.As when they need a specific response.
func DecodeJSON(writer http.ResponseWriter, request *http.Request, maxBytes int64, destination any) error {
	if writer == nil {
		return fmt.Errorf("response writer is required")
	}
	if request == nil || request.Body == nil {
		return fmt.Errorf("request body is required")
	}
	if maxBytes <= 0 {
		return fmt.Errorf("maximum JSON body size must be positive")
	}

	request.Body = http.MaxBytesReader(writer, request.Body, maxBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("decode JSON body: %w", err)
	}

	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return fmt.Errorf("decode trailing JSON data: %w", err)
	}
	return ErrMultipleJSONValues
}
