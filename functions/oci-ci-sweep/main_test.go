package main

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

// fakeServiceError implements common.ServiceError so isConflictError can be
// tested without a live OCI response. msg is what Error() returns, kept
// separate from status/code so a test can exercise the structured checks
// (GetHTTPStatusCode / GetCode) without the fallback string match also firing.
type fakeServiceError struct {
	status int
	code   string
	msg    string
}

func (e fakeServiceError) Error() string           { return e.msg }
func (e fakeServiceError) GetHTTPStatusCode() int  { return e.status }
func (e fakeServiceError) GetMessage() string      { return e.msg }
func (e fakeServiceError) GetCode() string         { return e.code }
func (e fakeServiceError) GetOpcRequestID() string { return "" }

func TestIsConflictError(t *testing.T) {
	// msg deliberately avoids the fallback substrings so these two isolate the
	// structured ServiceError path.
	structured409 := fakeServiceError{status: http.StatusConflict, code: "LimitExceeded", msg: "request rejected"}
	structuredIncorrectState := fakeServiceError{status: http.StatusBadRequest, code: "IncorrectState", msg: "request rejected"}

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"structured 409", structured409, true},
		{"structured IncorrectState", structuredIncorrectState, true},
		{"wrapped structured 409", fmt.Errorf("deleting cluster: %w", structured409), true},
		{"string 409", errors.New("Error returned by Service. Http Status Code: 409"), true},
		{"string already terminating", errors.New("resource is already terminating"), true},
		{"string IncorrectState", errors.New("IncorrectState: cannot delete"), true},
		{"unrelated 404", fakeServiceError{status: http.StatusNotFound, code: "NotFound", msg: "not found"}, false},
		// A structured non-conflict error must not be reclassified by the string
		// fallback just because its message/OPC request ID contains "409".
		{"structured 500 with 409 in message", fakeServiceError{status: http.StatusInternalServerError, code: "InternalServerError", msg: "opc-request-id 409ab/..."}, false},
		{"unrelated string", errors.New("connection refused"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isConflictError(tt.err); got != tt.want {
				t.Errorf("isConflictError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestIsNotFoundError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"structured 404", fakeServiceError{status: http.StatusNotFound, code: "NotAuthorizedOrNotFound", msg: "request rejected"}, true},
		{"wrapped structured 404", fmt.Errorf("fetching cluster: %w", fakeServiceError{status: http.StatusNotFound, code: "NotFound", msg: "gone"}), true},
		{"string 404", errors.New("Error returned by Service. Http Status Code: 404"), true},
		{"string NotFound", errors.New("resource NotFound"), true},
		// A structured non-404 error must not be reclassified as "gone" by the
		// string fallback just because its message/OPC request ID contains "404".
		{"structured 500 with 404 in message", fakeServiceError{status: http.StatusInternalServerError, code: "InternalServerError", msg: "opc-request-id 404ab/..."}, false},
		{"structured 409", fakeServiceError{status: http.StatusConflict, code: "IncorrectState", msg: "busy"}, false},
		{"unrelated string", errors.New("connection refused"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNotFoundError(tt.err); got != tt.want {
				t.Errorf("isNotFoundError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
