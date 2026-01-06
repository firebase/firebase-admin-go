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
	"net"
	"net/http"
	"net/url"
	"os"
	"syscall"

	"firebase.google.com/go/v4/errorutils"
)

// ErrorCode alias to errorutils.ErrorCode
type ErrorCode = errorutils.ErrorCode

// Error code constants aliases to errorutils
const (
	InvalidArgument    = errorutils.InvalidArgument
	FailedPrecondition = errorutils.FailedPrecondition
	OutOfRange         = errorutils.OutOfRange
	Unauthenticated    = errorutils.Unauthenticated
	PermissionDenied   = errorutils.PermissionDenied
	NotFound           = errorutils.NotFound
	Conflict           = errorutils.Conflict
	Aborted            = errorutils.Aborted
	AlreadyExists      = errorutils.AlreadyExists
	ResourceExhausted  = errorutils.ResourceExhausted
	Cancelled          = errorutils.Cancelled
	DataLoss           = errorutils.DataLoss
	Unknown            = errorutils.Unknown
	Internal           = errorutils.Internal
	Unavailable        = errorutils.Unavailable
	DeadlineExceeded   = errorutils.DeadlineExceeded
)

// FirebaseError is an alias to errorutils.FirebaseError for backwards compatibility.
type FirebaseError = errorutils.FirebaseError

// HasPlatformErrorCode checks if the given error contains a specific error code.
func HasPlatformErrorCode(err error, code ErrorCode) bool {
	return errorutils.HasPlatformErrorCode(err, code)
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

// NewFirebaseError creates a new error from the given HTTP response.
func NewFirebaseError(resp *Response) *FirebaseError {
	code, ok := httpStatusToErrorCodes[resp.Status]
	if !ok {
		code = Unknown
	}

	return &FirebaseError{
		ErrorCode: code,
		Message:   fmt.Sprintf("unexpected http response with status: %d\n%s", resp.Status, string(resp.Body)),
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
		base.ErrorCode = ErrorCode(gcpError.Error.Status)
	}

	if gcpError.Error.Message != "" {
		base.Message = gcpError.Error.Message
	}

	return base
}

func newFirebaseErrorTransport(err error) *FirebaseError {
	var code ErrorCode
	var msg string
	if os.IsTimeout(err) {
		code = DeadlineExceeded
		msg = fmt.Sprintf("timed out while making an http call: %v", err)
	} else if isConnectionRefused(err) {
		code = Unavailable
		msg = fmt.Sprintf("failed to establish a connection: %v", err)
	} else {
		code = Unknown
		msg = fmt.Sprintf("unknown error while making an http call: %v", err)
	}

	return &FirebaseError{
		ErrorCode: code,
		Message:   msg,
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
