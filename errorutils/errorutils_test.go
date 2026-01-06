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
	"net/http"
	"testing"
)

func TestFirebaseErrorImplementsError(t *testing.T) {
	fe := &FirebaseError{
		ErrorCode: NotFound,
		String:    "resource not found",
	}

	var err error = fe
	if err.Error() != "resource not found" {
		t.Errorf("Error() = %q; want = %q", err.Error(), "resource not found")
	}
}

func TestFirebaseErrorAccessors(t *testing.T) {
	resp := &http.Response{StatusCode: http.StatusNotFound}
	ext := map[string]interface{}{"key": "value"}

	fe := &FirebaseError{
		ErrorCode: NotFound,
		String:    "resource not found",
		Response:  resp,
		Ext:       ext,
	}

	if fe.Code() != NotFound {
		t.Errorf("Code() = %q; want = %q", fe.Code(), NotFound)
	}

	if fe.HTTPResponse() != resp {
		t.Errorf("HTTPResponse() = %v; want = %v", fe.HTTPResponse(), resp)
	}

	if fe.Extensions()["key"] != "value" {
		t.Errorf("Extensions()[\"key\"] = %v; want = %q", fe.Extensions()["key"], "value")
	}
}

func TestFirebaseErrorAccessorsWithNilFields(t *testing.T) {
	fe := &FirebaseError{
		ErrorCode: Internal,
		String:    "internal error",
	}

	if fe.HTTPResponse() != nil {
		t.Errorf("HTTPResponse() = %v; want = nil", fe.HTTPResponse())
	}

	if fe.Extensions() != nil {
		t.Errorf("Extensions() = %v; want = nil", fe.Extensions())
	}
}

func TestHasPlatformErrorCode(t *testing.T) {
	fe := &FirebaseError{
		ErrorCode: NotFound,
		String:    "not found",
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
		String:    "internal error",
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
		name     string
		code     ErrorCode
		checkFn  func(error) bool
		wantTrue bool
	}{
		{"InvalidArgument", InvalidArgument, IsInvalidArgument, true},
		{"FailedPrecondition", FailedPrecondition, IsFailedPrecondition, true},
		{"OutOfRange", OutOfRange, IsOutOfRange, true},
		{"Unauthenticated", Unauthenticated, IsUnauthenticated, true},
		{"PermissionDenied", PermissionDenied, IsPermissionDenied, true},
		{"NotFound", NotFound, IsNotFound, true},
		{"Conflict", Conflict, IsConflict, true},
		{"Aborted", Aborted, IsAborted, true},
		{"AlreadyExists", AlreadyExists, IsAlreadyExists, true},
		{"ResourceExhausted", ResourceExhausted, IsResourceExhausted, true},
		{"Cancelled", Cancelled, IsCancelled, true},
		{"DataLoss", DataLoss, IsDataLoss, true},
		{"Unknown", Unknown, IsUnknown, true},
		{"Internal", Internal, IsInternal, true},
		{"Unavailable", Unavailable, IsUnavailable, true},
		{"DeadlineExceeded", DeadlineExceeded, IsDeadlineExceeded, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fe := &FirebaseError{
				ErrorCode: tc.code,
				String:    "test error",
			}

			if tc.checkFn(fe) != tc.wantTrue {
				t.Errorf("%s check = %v; want = %v", tc.name, tc.checkFn(fe), tc.wantTrue)
			}
		})
	}
}

func TestIsErrorCodeFunctionsWithWrongCode(t *testing.T) {
	fe := &FirebaseError{
		ErrorCode: NotFound,
		String:    "not found",
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
	// Verify error codes have the expected string values
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
