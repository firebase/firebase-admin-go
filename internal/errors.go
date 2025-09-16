// Copyright 2020 Google LLC All Rights Reserved.
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
	"net"
	"net/http"
	"net/url"
	"os"
	"syscall"

	"firebase.google.com/go/v4/errorutils"
)

// FirebaseError is an error type containing an error code string.
type FirebaseError struct {
	ErrorCode string
	String    string
	Response  *http.Response
	Ext       map[string]interface{}
}

func (fe *FirebaseError) Error() string {
	return fe.String
}

// PlatformCode returns the platform-level error code.
func (fe *FirebaseError) PlatformCode() string {
	return fe.ErrorCode
}

// ServiceCode returns the service-specific error code.
func (fe *FirebaseError) ServiceCode() string {
	if code, ok := fe.Ext["authErrorCode"].(string); ok {
		return code
	}
	return ""
}

// Message returns the human-readable error message.
func (fe *FirebaseError) Message() string {
	return fe.String
}

// HTTPResponse returns the original HTTP response.
func (fe *FirebaseError) HTTPResponse() *http.Response {
	return fe.Response
}

// HasPlatformErrorCode checks if the given error contains a specific error code.
func HasPlatformErrorCode(err error, code string) bool {
	fe, ok := err.(*FirebaseError)
	return ok && fe.ErrorCode == code
}

var httpStatusToErrorCodes = map[int]string{
	http.StatusBadRequest:          errorutils.InvalidArgument,
	http.StatusUnauthorized:        errorutils.Unauthenticated,
	http.StatusForbidden:           errorutils.PermissionDenied,
	http.StatusNotFound:            errorutils.NotFound,
	http.StatusConflict:            errorutils.Conflict,
	http.StatusTooManyRequests:     errorutils.ResourceExhausted,
	http.StatusInternalServerError: errorutils.Internal,
	http.StatusServiceUnavailable:  errorutils.Unavailable,
}

// NewFirebaseError creates a new error from the given HTTP response.
func NewFirebaseError(resp *Response) *FirebaseError {
	code, ok := httpStatusToErrorCodes[resp.Status]
	if !ok {
		code = errorutils.Unknown
	}

	return &FirebaseError{
		ErrorCode: code,
		String:    fmt.Sprintf("unexpected http response with status: %d\n%s", resp.Status, string(resp.Body)),
		Response:  resp.LowLevelResponse(),
		Ext:       make(map[string]interface{}),
	}
}

// NewFirebaseErrorOnePlatform parses the response payload as a GCP error response
// and create an error from the details extracted.
//
// If the response failes to parse, or otherwise doesn't provide any useful details
// NewFirebaseErrorOnePlatform creates an error with some sensible defaults.
func NewFirebaseErrorOnePlatform(resp *Response) *FirebaseError {
	base := NewFirebaseError(resp)

	var gcpError struct {
		Error struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		} `json:"error"`
	}
	json.Unmarshal(resp.Body, &gcpError) // ignore any json parse errors at this level
	if gcpError.Error.Status != "" {
		base.ErrorCode = gcpError.Error.Status
	}

	if gcpError.Error.Message != "" {
		base.String = gcpError.Error.Message
	}

	return base
}

func newFirebaseErrorTransport(err error) *FirebaseError {
	var code string
	var msg string
	if os.IsTimeout(err) {
		code = errorutils.DeadlineExceeded
		msg = fmt.Sprintf("timed out while making an http call: %v", err)
	} else if isConnectionRefused(err) {
		code = errorutils.Unavailable
		msg = fmt.Sprintf("failed to establish a connection: %v", err)
	} else {
		code = errorutils.Unknown
		msg = fmt.Sprintf("unknown error while making an http call: %v", err)
	}

	return &FirebaseError{
		ErrorCode: code,
		String:    msg,
		Ext:       make(map[string]interface{}),
	}
}

// isConnectionRefused attempts to determine if the given error was caused by a failure to establish a
// connection.
//
// A net.OpError where the Op field is set to "dial" or "read" is considered a connection refused
// error. Similarly an ECONNREFUSED error code (Linux-specific) is also considered a connection
// refused error.
func isConnectionRefused(err error) bool {
	switch t := err.(type) {
	case *url.Error:
		return isConnectionRefused(t.Err)
	case *net.OpError:
		if t.Op == "dial" || t.Op == "read" {
			return true
		}
		return isConnectionRefused(t.Err)
	case syscall.Errno:
		return t == syscall.ECONNREFUSED
	}

	return false
}
