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
	"errors"
	"net/http"
)

// ErrorCode represents the platform-wide error codes that can be raised by
// Admin SDK APIs.
type ErrorCode string

const (
	// InvalidArgument is a OnePlatform error code.
	InvalidArgument ErrorCode = "INVALID_ARGUMENT"

	// FailedPrecondition is a OnePlatform error code.
	FailedPrecondition ErrorCode = "FAILED_PRECONDITION"

	// OutOfRange is a OnePlatform error code.
	OutOfRange ErrorCode = "OUT_OF_RANGE"

	// Unauthenticated is a OnePlatform error code.
	Unauthenticated ErrorCode = "UNAUTHENTICATED"

	// PermissionDenied is a OnePlatform error code.
	PermissionDenied ErrorCode = "PERMISSION_DENIED"

	// NotFound is a OnePlatform error code.
	NotFound ErrorCode = "NOT_FOUND"

	// Conflict is a custom error code that represents HTTP 409 responses.
	//
	// OnePlatform APIs typically respond with ABORTED or ALREADY_EXISTS explicitly. But a few
	// old APIs send HTTP 409 Conflict without any additional details to distinguish between the two
	// cases. For these we currently use this error code. As more APIs adopt OnePlatform conventions
	// this will become less important.
	Conflict ErrorCode = "CONFLICT"

	// Aborted is a OnePlatform error code.
	Aborted ErrorCode = "ABORTED"

	// AlreadyExists is a OnePlatform error code.
	AlreadyExists ErrorCode = "ALREADY_EXISTS"

	// ResourceExhausted is a OnePlatform error code.
	ResourceExhausted ErrorCode = "RESOURCE_EXHAUSTED"

	// Cancelled is a OnePlatform error code.
	Cancelled ErrorCode = "CANCELLED"

	// DataLoss is a OnePlatform error code.
	DataLoss ErrorCode = "DATA_LOSS"

	// Unknown is a OnePlatform error code.
	Unknown ErrorCode = "UNKNOWN"

	// Internal is a OnePlatform error code.
	Internal ErrorCode = "INTERNAL"

	// Unavailable is a OnePlatform error code.
	Unavailable ErrorCode = "UNAVAILABLE"

	// DeadlineExceeded is a OnePlatform error code.
	DeadlineExceeded ErrorCode = "DEADLINE_EXCEEDED"
)

// FirebaseError is an error type containing an:
// - error code string.
// - error message.
// - HTTP response that caused this error, if any.
// - additional metadata about the error.
type FirebaseError struct {
	ErrorCode ErrorCode
	Message   string
	Response  *http.Response
	Ext       map[string]interface{}
}

// Error implements the error interface.
func (fe *FirebaseError) Error() string {
	return fe.Message
}

// Code returns the canonical error code associated with this error.
func (fe *FirebaseError) Code() ErrorCode {
	return fe.ErrorCode
}

// HTTPResponse returns the HTTP response that caused this error, if any.
// Returns nil if the error was not caused by an HTTP response.
func (fe *FirebaseError) HTTPResponse() *http.Response {
	return fe.Response
}

// Extensions returns additional metadata associated with this error.
// Returns nil if no extensions are available.
func (fe *FirebaseError) Extensions() map[string]interface{} {
	return fe.Ext
}

// IsInvalidArgument checks if the given error was due to an invalid client argument.
func IsInvalidArgument(err error) bool {
	return HasPlatformErrorCode(err, InvalidArgument)
}

// IsFailedPrecondition checks if the given error was because a request could not be executed
// in the current system state, such as deleting a non-empty directory.
func IsFailedPrecondition(err error) bool {
	return HasPlatformErrorCode(err, FailedPrecondition)
}

// IsOutOfRange checks if the given error due to an invalid range specified by the client.
func IsOutOfRange(err error) bool {
	return HasPlatformErrorCode(err, OutOfRange)
}

// IsUnauthenticated checks if the given error was caused by an unauthenticated request.
//
// Unauthenticated requests are due to missing, invalid, or expired OAuth token.
func IsUnauthenticated(err error) bool {
	return HasPlatformErrorCode(err, Unauthenticated)
}

// IsPermissionDenied checks if the given error was due to a client not having sufficient
// permissions.
//
// This can happen because the OAuth token does not have the right scopes, the client doesn't have
// permission, or the API has not been enabled for the client project.
func IsPermissionDenied(err error) bool {
	return HasPlatformErrorCode(err, PermissionDenied)
}

// IsNotFound checks if the given error was due to a specified resource being not found.
//
// This may also occur when the request is rejected by undisclosed reasons, such as whitelisting.
func IsNotFound(err error) bool {
	return HasPlatformErrorCode(err, NotFound)
}

// IsConflict checks if the given error was due to a concurrency conflict, such as a
// read-modify-write conflict.
//
// This represents an HTTP 409 Conflict status code, without additional information to distinguish
// between ABORTED or ALREADY_EXISTS error conditions.
func IsConflict(err error) bool {
	return HasPlatformErrorCode(err, Conflict)
}

// IsAborted checks if the given error was due to a concurrency conflict, such as a
// read-modify-write conflict.
func IsAborted(err error) bool {
	return HasPlatformErrorCode(err, Aborted)
}

// IsAlreadyExists checks if the given error was because a resource that a client tried to create
// already exists.
func IsAlreadyExists(err error) bool {
	return HasPlatformErrorCode(err, AlreadyExists)
}

// IsResourceExhausted checks if the given error was caused by either running out of a quota or
// reaching a rate limit.
func IsResourceExhausted(err error) bool {
	return HasPlatformErrorCode(err, ResourceExhausted)
}

// IsCancelled checks if the given error was due to the client cancelling a request.
func IsCancelled(err error) bool {
	return HasPlatformErrorCode(err, Cancelled)
}

// IsDataLoss checks if the given error was due to an unrecoverable data loss or corruption.
//
// The client should report such errors to the end user.
func IsDataLoss(err error) bool {
	return HasPlatformErrorCode(err, DataLoss)
}

// IsUnknown checks if the given error was caused by an unknown server error.
//
// This typically indicates a server bug.
func IsUnknown(err error) bool {
	return HasPlatformErrorCode(err, Unknown)
}

// IsInternal checks if the given error was due to an internal server error.
//
// This typically indicates a server bug.
func IsInternal(err error) bool {
	return HasPlatformErrorCode(err, Internal)
}

// IsUnavailable checks if the given error was caused by an unavailable service.
//
// This typically indicates that the target service is temporarily down.
func IsUnavailable(err error) bool {
	return HasPlatformErrorCode(err, Unavailable)
}

// IsDeadlineExceeded checks if the given error was due a request exceeding a deadline.
//
// This will happen only if the caller sets a deadline that is shorter than the method's default
// deadline (i.e. requested deadline is not enough for the server to process the request) and the
// request did not finish within the deadline.
func IsDeadlineExceeded(err error) bool {
	return HasPlatformErrorCode(err, DeadlineExceeded)
}

// HTTPResponse returns the http.Response instance that caused the given error.
//
// If the error was not caused by an HTTP error response, returns nil.
//
// Returns a buffered copy of the original response received from the network stack. It is safe to
// read the response content from the returned http.Response.
func HTTPResponse(err error) *http.Response {
	fe, ok := err.(*FirebaseError)
	if ok {
		return fe.Response
	}

	return nil
}

// HasPlatformErrorCode checks if the given error contains a specific error code.
func HasPlatformErrorCode(err error, code ErrorCode) bool {
	var fe *FirebaseError
	return errors.As(err, &fe) && fe.ErrorCode == code
}
