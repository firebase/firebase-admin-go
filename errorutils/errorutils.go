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

// Package errorutils provides functions for checking and handling error conditions.
package errorutils // import "firebase.google.com/go/v4/errorutils"

import "firebase.google.com/go/v4/internal"

import "net/http"

// IsInvalidArgument checks if the given error was due to an invalid client argument.
func IsInvalidArgument(err error) bool {
	return internal.HasPlatformErrorCode(err, internal.InvalidArgument)
}

// IsUnauthenticated checks if the given error was caused by an unauthenticated request.
//
// Unauthenticated requests are due to missing, invalid, or expired OAuth token.
func IsUnauthenticated(err error) bool {
	return internal.HasPlatformErrorCode(err, internal.Unauthenticated)
}

// IsPermissionDenied checks if the given error was due to a client not having suffificient
// permissions.
//
// This can happen because the OAuth token does not have the right scopes, the client doesn't have
// permission, or the API has not been enabled for the client project.
func IsPermissionDenied(err error) bool {
	return internal.HasPlatformErrorCode(err, internal.PermissionDenied)
}

// IsNotFound checks if the given error was due to a specified resource being not found.
//
// This may also occur when the request is rejected by undisclosed reasons, such as whitelisting.
func IsNotFound(err error) bool {
	return internal.HasPlatformErrorCode(err, internal.NotFound)
}

// IsConflict checks if the given error was due to a concurrency conflict, such as a
// read-modify-write conflict.
func IsConflict(err error) bool {
	return internal.HasPlatformErrorCode(err, internal.Conflict)
}

// IsResourceExhausted checks if the given error was caused by either running out of a quota or
// reaching a rate limit.
func IsResourceExhausted(err error) bool {
	return internal.HasPlatformErrorCode(err, internal.ResourceExhausted)
}

// IsInternal checks if the given error was due to an internal server error.
//
// This typically indicates a server bug.
func IsInternal(err error) bool {
	return internal.HasPlatformErrorCode(err, internal.Internal)
}

// IsUnavailable checks if the given error was caused by an unavailable service.
//
// This typically indicates that the target service is temporarily down.
func IsUnavailable(err error) bool {
	return internal.HasPlatformErrorCode(err, internal.Unavailable)
}

// IsUnknown checks if the given error was cuased by an unknown server error.
//
// This typically indicates a server bug.
func IsUnknown(err error) bool {
	return internal.HasPlatformErrorCode(err, internal.Unknown)
}

// HTTPResponse returns the http.Response instance that caused the given error.
//
// If the error was not caused by an HTTP error response, returns nil.
//
// Returns a buffered copy of the original response received from the network stack. It is safe to
// read the response content from the returned http.Response.
func HTTPResponse(err error) *http.Response {
	fe, ok := err.(*internal.FirebaseError)
	if ok {
		return fe.Response
	}

	return nil
}
