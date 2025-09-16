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

// Package errorutils provides functions for checking and handling error conditions.
package errorutils

import (
	"net/http"
)

// FirebaseError is an interface that all Firebase errors (both internal SDK errors and mock
// test errors) satisfy.
type FirebaseError interface {
	Error() string
	// PlatformCode returns the broad platform-level error code (e.g. "NOT_FOUND", "INVALID_ARGUMENT").
	PlatformCode() string
	// ServiceCode returns the service-specific error code (e.g. "EMAIL_EXISTS", "USER_NOT_FOUND").
	ServiceCode() string
	// Message returns the human-readable error message.
	Message() string
}

const (
	// InvalidArgument is a OnePlatform error code.
	InvalidArgument = "INVALID_ARGUMENT"

	// FailedPrecondition is a OnePlatform error code.
	FailedPrecondition = "FAILED_PRECONDITION"

	// OutOfRange is a OnePlatform error code.
	OutOfRange = "OUT_OF_RANGE"

	// Unauthenticated is a OnePlatform error code.
	Unauthenticated = "UNAUTHENTICATED"

	// PermissionDenied is a OnePlatform error code.
	PermissionDenied = "PERMISSION_DENIED"

	// NotFound is a OnePlatform error code.
	NotFound = "NOT_FOUND"

	// Conflict is a custom error code that represents HTTP 409 responses.
	//
	// OnePlatform APIs typically respond with ABORTED or ALREADY_EXISTS explicitly. But a few
	// old APIs send HTTP 409 Conflict without any additional details to distinguish between the two
	// cases. For these we currently use this error code. As more APIs adopt OnePlatform conventions
	// this will become less important.
	Conflict = "CONFLICT"

	// Aborted is a OnePlatform error code.
	Aborted = "ABORTED"

	// AlreadyExists is a OnePlatform error code.
	AlreadyExists = "ALREADY_EXISTS"

	// ResourceExhausted is a OnePlatform error code.
	ResourceExhausted = "RESOURCE_EXHAUSTED"

	// Cancelled is a OnePlatform error code.
	Cancelled = "CANCELLED"

	// DataLoss is a OnePlatform error code.
	DataLoss = "DATA_LOSS"

	// Unknown is a OnePlatform error code.
	Unknown = "UNKNOWN"

	// Internal is a OnePlatform error code.
	Internal = "INTERNAL"

	// Unavailable is a OnePlatform error code.
	Unavailable = "UNAVAILABLE"

	// DeadlineExceeded is a OnePlatform error code.
	DeadlineExceeded = "DEADLINE_EXCEEDED"
)

// NewTestError creates a mock error for unit testing.
//
// The returned error implements the FirebaseError interface, and will be correctly recognized by
// all SDK error-checking functions (e.g. IsNotFound, auth.IsEmailAlreadyExists).
func NewTestError(platformCode, serviceCode, message string) error {
	return &testError{
		pCode: platformCode,
		sCode: serviceCode,
		msg:   message,
	}
}

// testError is an unexported type that implements the public FirebaseError interface.
type testError struct {
	pCode string
	sCode string
	msg   string
}

func (e *testError) Error() string {
	return e.msg
}

func (e *testError) PlatformCode() string {
	return e.pCode
}

func (e *testError) ServiceCode() string {
	return e.sCode
}

func (e *testError) Message() string {
	return e.msg
}

// IsInvalidArgument checks if the given error was due to an invalid client argument.
func IsInvalidArgument(err error) bool {
	if fe, ok := err.(FirebaseError); ok {
		return fe.PlatformCode() == InvalidArgument
	}
	return false
}

// IsFailedPrecondition checks if the given error was because a request could not be executed
// in the current system state, such as deleting a non-empty directory.
func IsFailedPrecondition(err error) bool {
	if fe, ok := err.(FirebaseError); ok {
		return fe.PlatformCode() == FailedPrecondition
	}
	return false
}

// IsOutOfRange checks if the given error due to an invalid range specified by the client.
func IsOutOfRange(err error) bool {
	if fe, ok := err.(FirebaseError); ok {
		return fe.PlatformCode() == OutOfRange
	}
	return false
}

// IsUnauthenticated checks if the given error was caused by an unauthenticated request.
//
// Unauthenticated requests are due to missing, invalid, or expired OAuth token.
func IsUnauthenticated(err error) bool {
	if fe, ok := err.(FirebaseError); ok {
		return fe.PlatformCode() == Unauthenticated
	}
	return false
}

// IsPermissionDenied checks if the given error was due to a client not having suffificient
// permissions.
//
// This can happen because the OAuth token does not have the right scopes, the client doesn't have
// permission, or the API has not been enabled for the client project.
func IsPermissionDenied(err error) bool {
	if fe, ok := err.(FirebaseError); ok {
		return fe.PlatformCode() == PermissionDenied
	}
	return false
}

// IsNotFound checks if the given error was due to a specified resource being not found.
//
// This may also occur when the request is rejected by undisclosed reasons, such as whitelisting.
func IsNotFound(err error) bool {
	if fe, ok := err.(FirebaseError); ok {
		return fe.PlatformCode() == NotFound
	}
	return false
}

// IsConflict checks if the given error was due to a concurrency conflict, such as a
// read-modify-write conflict.
//
// This represents an HTTP 409 Conflict status code, without additional information to distinguish
// between ABORTED or ALREADY_EXISTS error conditions.
func IsConflict(err error) bool {
	if fe, ok := err.(FirebaseError); ok {
		return fe.PlatformCode() == Conflict
	}
	return false
}

// IsAborted checks if the given error was due to a concurrency conflict, such as a
// read-modify-write conflict.
func IsAborted(err error) bool {
	if fe, ok := err.(FirebaseError); ok {
		return fe.PlatformCode() == Aborted
	}
	return false
}

// IsAlreadyExists checks if the given error was because a resource that a client tried to create
// already exists.
func IsAlreadyExists(err error) bool {
	if fe, ok := err.(FirebaseError); ok {
		return fe.PlatformCode() == AlreadyExists
	}
	return false
}

// IsResourceExhausted checks if the given error was caused by either running out of a quota or
// reaching a rate limit.
func IsResourceExhausted(err error) bool {
	if fe, ok := err.(FirebaseError); ok {
		return fe.PlatformCode() == ResourceExhausted
	}
	return false
}

// IsCancelled checks if the given error was due to the client cancelling a request.
func IsCancelled(err error) bool {
	if fe, ok := err.(FirebaseError); ok {
		return fe.PlatformCode() == Cancelled
	}
	return false
}

// IsDataLoss checks if the given error was due to an unrecoverable data loss or corruption.
//
// The client should report such errors to the end user.
func IsDataLoss(err error) bool {
	if fe, ok := err.(FirebaseError); ok {
		return fe.PlatformCode() == DataLoss
	}
	return false
}

// IsUnknown checks if the given error was cuased by an unknown server error.
//
// This typically indicates a server bug.
func IsUnknown(err error) bool {
	if fe, ok := err.(FirebaseError); ok {
		return fe.PlatformCode() == Unknown
	}
	return false
}

// IsInternal checks if the given error was due to an internal server error.
//
// This typically indicates a server bug.
func IsInternal(err error) bool {
	if fe, ok := err.(FirebaseError); ok {
		return fe.PlatformCode() == Internal
	}
	return false
}

// IsUnavailable checks if the given error was caused by an unavailable service.
//
// This typically indicates that the target service is temporarily down.
func IsUnavailable(err error) bool {
	if fe, ok := err.(FirebaseError); ok {
		return fe.PlatformCode() == Unavailable
	}
	return false
}

// IsDeadlineExceeded checks if the given error was due a request exceeding a deadline.
//
// This will happen only if the caller sets a deadline that is shorter than the method's default
// deadline (i.e. requested deadline is not enough for the server to process the request) and the
// request did not finish within the deadline.
func IsDeadlineExceeded(err error) bool {
	if fe, ok := err.(FirebaseError); ok {
		return fe.PlatformCode() == DeadlineExceeded
	}
	return false
}

// HTTPError is an interface for errors that are caused by HTTP responses.
type HTTPError interface {
	HTTPResponse() *http.Response
}

// HTTPResponse returns the http.Response instance that caused the given error.
//
// If the error was not caused by an HTTP error response, returns nil.
//
// Returns a buffered copy of the original response received from the network stack. It is safe to
// read the response content from the returned http.Response.
func HTTPResponse(err error) *http.Response {
	if he, ok := err.(HTTPError); ok {
		return he.HTTPResponse()
	}
	return nil
}
