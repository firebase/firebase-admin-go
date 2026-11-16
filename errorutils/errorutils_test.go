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

package errorutils

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestFirebaseErrorImplementsError(t *testing.T) {
	fe := &FirebaseError{
		ErrorCode: NotFound,
		Message:   "resource not found",
	}

	var err error = fe
	if err.Error() != "resource not found" {
		t.Errorf("Error() = %q; want = %q", err.Error(), "resource not found")
	}
}

func TestHTTPResponseFunctionWithNilResponseField(t *testing.T) {
	fe := &FirebaseError{
		ErrorCode: Internal,
		Message:   "internal error",
		Response:  nil,
	}

	if HTTPResponse(fe) != nil {
		t.Errorf("HTTPResponse(fe) = %v; want = nil", HTTPResponse(fe))
	}
}

func TestHasPlatformErrorCode(t *testing.T) {
	fe := &FirebaseError{
		ErrorCode: NotFound,
		Message:   "not found",
	}

	if !HasPlatformErrorCode(fe, NotFound) {
		t.Error("HasPlatformErrorCode(fe, NotFound) = false; want = true")
	}

	if HasPlatformErrorCode(fe, Internal) {
		t.Error("HasPlatformErrorCode(fe, Internal) = true; want = false")
	}
}

func TestHasPlatformErrorCodeWithNonFirebaseError(t *testing.T) {
	err := errors.New("regular error")

	if HasPlatformErrorCode(err, NotFound) {
		t.Error("HasPlatformErrorCode(err, NotFound) = true; want = false")
	}
}

func TestHasPlatformErrorCodeWithNil(t *testing.T) {
	if HasPlatformErrorCode(nil, NotFound) {
		t.Error("HasPlatformErrorCode(nil, NotFound) = true; want = false")
	}
}

func TestHTTPResponseFunction(t *testing.T) {
	resp := &http.Response{StatusCode: http.StatusInternalServerError}
	fe := &FirebaseError{
		ErrorCode: Internal,
		Message:   "internal error",
		Response:  resp,
	}

	if HTTPResponse(fe) != resp {
		t.Errorf("HTTPResponse(fe) = %v; want = %v", HTTPResponse(fe), resp)
	}
}

func TestHTTPResponseFunctionWithNonFirebaseError(t *testing.T) {
	err := errors.New("regular error")

	if HTTPResponse(err) != nil {
		t.Errorf("HTTPResponse(err) = %v; want = nil", HTTPResponse(err))
	}
}

func TestHTTPResponseFunctionWithNil(t *testing.T) {
	if HTTPResponse(nil) != nil {
		t.Errorf("HTTPResponse(nil) = %v; want = nil", HTTPResponse(nil))
	}
}

func TestIsErrorCodeFunctions(t *testing.T) {
	testCases := []struct {
		name    string
		code    ErrorCode
		checkFn func(error) bool
	}{
		{"IsInvalidArgument", InvalidArgument, IsInvalidArgument},
		{"IsFailedPrecondition", FailedPrecondition, IsFailedPrecondition},
		{"IsOutOfRange", OutOfRange, IsOutOfRange},
		{"IsUnauthenticated", Unauthenticated, IsUnauthenticated},
		{"IsPermissionDenied", PermissionDenied, IsPermissionDenied},
		{"IsNotFound", NotFound, IsNotFound},
		{"IsConflict", Conflict, IsConflict},
		{"IsAborted", Aborted, IsAborted},
		{"IsAlreadyExists", AlreadyExists, IsAlreadyExists},
		{"IsResourceExhausted", ResourceExhausted, IsResourceExhausted},
		{"IsCancelled", Cancelled, IsCancelled},
		{"IsDataLoss", DataLoss, IsDataLoss},
		{"IsUnknown", Unknown, IsUnknown},
		{"IsInternal", Internal, IsInternal},
		{"IsUnavailable", Unavailable, IsUnavailable},
		{"IsDeadlineExceeded", DeadlineExceeded, IsDeadlineExceeded},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fe := &FirebaseError{
				ErrorCode: tc.code,
				Message:   "test error",
			}

			if !tc.checkFn(fe) {
				t.Errorf("%s check = false; want = true", tc.name)
			}
		})
	}
}

func TestIsErrorCodeFunctionsWithWrongCode(t *testing.T) {
	fe := &FirebaseError{
		ErrorCode: NotFound,
		Message:   "not found",
	}

	checks := []struct {
		name    string
		checkFn func(error) bool
	}{
		{"IsInvalidArgument", IsInvalidArgument},
		{"IsFailedPrecondition", IsFailedPrecondition},
		{"IsOutOfRange", IsOutOfRange},
		{"IsUnauthenticated", IsUnauthenticated},
		{"IsPermissionDenied", IsPermissionDenied},
		{"IsConflict", IsConflict},
		{"IsAborted", IsAborted},
		{"IsAlreadyExists", IsAlreadyExists},
		{"IsResourceExhausted", IsResourceExhausted},
		{"IsCancelled", IsCancelled},
		{"IsDataLoss", IsDataLoss},
		{"IsUnknown", IsUnknown},
		{"IsInternal", IsInternal},
		{"IsUnavailable", IsUnavailable},
		{"IsDeadlineExceeded", IsDeadlineExceeded},
	}

	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			if tc.checkFn(fe) {
				t.Errorf("%s(NotFoundError) = true; want = false", tc.name)
			}
		})
	}
}

func TestIsErrorCodeFunctionsWithNonFirebaseError(t *testing.T) {
	err := errors.New("regular error")

	checks := []struct {
		name    string
		checkFn func(error) bool
	}{
		{"IsInvalidArgument", IsInvalidArgument},
		{"IsNotFound", IsNotFound},
		{"IsInternal", IsInternal},
	}

	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			if tc.checkFn(err) {
				t.Errorf("%s(regularError) = true; want = false", tc.name)
			}
		})
	}
}

func TestIsErrorCodeFunctionsWithNil(t *testing.T) {
	if IsNotFound(nil) {
		t.Error("IsNotFound(nil) = true; want = false")
	}

	if IsInternal(nil) {
		t.Error("IsInternal(nil) = true; want = false")
	}
}

func TestErrorCodeValues(t *testing.T) {
	// These values are part of the public API contract with OnePlatform
	testCases := []struct {
		code ErrorCode
		want string
	}{
		{InvalidArgument, "INVALID_ARGUMENT"},
		{FailedPrecondition, "FAILED_PRECONDITION"},
		{OutOfRange, "OUT_OF_RANGE"},
		{Unauthenticated, "UNAUTHENTICATED"},
		{PermissionDenied, "PERMISSION_DENIED"},
		{NotFound, "NOT_FOUND"},
		{Conflict, "CONFLICT"},
		{Aborted, "ABORTED"},
		{AlreadyExists, "ALREADY_EXISTS"},
		{ResourceExhausted, "RESOURCE_EXHAUSTED"},
		{Cancelled, "CANCELLED"},
		{DataLoss, "DATA_LOSS"},
		{Unknown, "UNKNOWN"},
		{Internal, "INTERNAL"},
		{Unavailable, "UNAVAILABLE"},
		{DeadlineExceeded, "DEADLINE_EXCEEDED"},
	}

	for _, tc := range testCases {
		t.Run(tc.want, func(t *testing.T) {
			if string(tc.code) != tc.want {
				t.Errorf("ErrorCode = %q; want = %q", tc.code, tc.want)
			}
		})
	}
}

func TestWrappedFirebaseErrorNotDetected(t *testing.T) {
	// Documents current behavior: wrapped errors are NOT detected by IsXxx functions
	// This is a known limitation - the SDK uses type assertion, not errors.As
	fe := &FirebaseError{
		ErrorCode: NotFound,
		Message:   "not found",
	}
	wrapped := fmt.Errorf("wrapped: %w", fe)

	// Current behavior: wrapped errors return false
	if IsNotFound(wrapped) {
		t.Error("IsNotFound(wrapped) = true; current implementation should return false for wrapped errors")
	}

	if HasPlatformErrorCode(wrapped, NotFound) {
		t.Error("HasPlatformErrorCode(wrapped) = true; current implementation should return false for wrapped errors")
	}
}
