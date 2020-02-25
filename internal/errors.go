// Copyright 2020 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// ErrorCode represents the platform-wide error codes that can be raised by
// Admin SDK APIs.
type ErrorCode string

const (
	InvalidArgument    ErrorCode = "INVALID_ARGUMENT"
	FailedPrecondition ErrorCode = "FAILED_PRECONDITION"
	OutOfRange         ErrorCode = "OUT_OF_RANGE"
	Unauthenticated    ErrorCode = "UNAUTHENTICATED"
	PermissionDenied   ErrorCode = "PERMISSION_DENIED"
	NotFound           ErrorCode = "NOT_FOUND"
	Conflict           ErrorCode = "CONFLICT"
	Aborted            ErrorCode = "ABORTED"
	ResourceExhausted  ErrorCode = "RESOURCE_EXHAUSTED"
	Cancelled          ErrorCode = "CANCELLED"
	DataLoss           ErrorCode = "DATA_LOSS"
	Unknown            ErrorCode = "UNKNOWN"
	Internal           ErrorCode = "INTERNAL"
	Unavailable        ErrorCode = "UNAVAILABLE"
	DeadlineExceeded   ErrorCode = "DEADLINE_EXCEEDED"
)

// FirebaseError is an error type containing an error code string.
type FirebaseError struct {
	ErrorCode ErrorCode
	Code      string
	String    string
	Response  *http.Response
}

func (fe *FirebaseError) Error() string {
	return fe.String
}

func HasPlatformErrorCode(err error, code ErrorCode) bool {
	fe, ok := err.(*FirebaseError)
	return ok && fe.ErrorCode == code
}

// HasErrorCode checks if the given error contain a specific error code.
func HasErrorCode(err error, code string) bool {
	fe, ok := err.(*FirebaseError)
	return ok && fe.Code == code
}

// Error creates a new FirebaseError from the specified error code and message.
func Error(code string, msg string) *FirebaseError {
	return &FirebaseError{
		Code:   code,
		String: msg,
	}
}

// Errorf creates a new FirebaseError from the specified error code and message.
func Errorf(code string, msg string, args ...interface{}) *FirebaseError {
	return Error(code, fmt.Sprintf(msg, args...))
}

var httpStatusToErrorCodes = map[int]ErrorCode{
	http.StatusBadRequest:          InvalidArgument,
	http.StatusUnauthorized:        Unauthenticated,
	http.StatusForbidden:           PermissionDenied,
	http.StatusNotFound:            NotFound,
	http.StatusConflict:            Conflict,
	http.StatusTooManyRequests:     ResourceExhausted,
	http.StatusInternalServerError: Internal,
	http.StatusServiceUnavailable:  Unavailable,
}

// NewFirebaseErrorFromHTTPResponse creates a new error from the given HTTP response.
func NewFirebaseErrorFromHTTPResponse(resp *Response) *FirebaseError {
	code, ok := httpStatusToErrorCodes[resp.Status]
	if !ok {
		code = Unknown
	}

	return &FirebaseError{
		ErrorCode: code,
		String:    fmt.Sprintf("unexpected http response with status: %d\n%s", resp.Status, string(resp.Body)),
		Response:  resp.LowLevelResponse(),
	}
}

// NewFirebaseErrorFromPlatformResponse parses the response payload as a GCP error response
// and create an error from the details extracted.
//
// If the response failes to parse, or otherwise doesn't provide any useful details
// CreatePlatformError creates an error with some sensible defaults.
func NewFirebaseErrorFromPlatformResponse(resp *Response) *FirebaseError {
	base := NewFirebaseErrorFromHTTPResponse(resp)

	var gcpError struct {
		Error struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		} `json:"error"`
	}
	json.Unmarshal(resp.Body, &gcpError) // ignore any json parse errors at this level
	if gcpError.Error.Status != "" {
		base.ErrorCode = ErrorCode(gcpError.Error.Status)
	}

	if gcpError.Error.Message != "" {
		base.String = gcpError.Error.Message
	}

	return base
}
