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

package errorutils // import "firebase.google.com/go/v4/errorutils"

import "firebase.google.com/go/v4/internal"

import "net/http"

func IsInvalidArgument(err error) bool {
	return hasPlatformErrorCode(err, internal.InvalidArgument)
}

func IsUnauthenticated(err error) bool {
	return hasPlatformErrorCode(err, internal.Unauthenticated)
}

func IsPermissionDenied(err error) bool {
	return hasPlatformErrorCode(err, internal.PermissionDenied)
}

func IsNotFound(err error) bool {
	return hasPlatformErrorCode(err, internal.NotFound)
}

func IsConflict(err error) bool {
	return hasPlatformErrorCode(err, internal.Conflict)
}

func IsResourceExhausted(err error) bool {
	return hasPlatformErrorCode(err, internal.ResourceExhausted)
}

func IsInternal(err error) bool {
	return hasPlatformErrorCode(err, internal.Internal)
}

func IsUnavailable(err error) bool {
	return hasPlatformErrorCode(err, internal.Unavailable)
}

func IsUnknown(err error) bool {
	return hasPlatformErrorCode(err, internal.Unknown)
}

func HTTPResponse(err error) *http.Response {
	fe, ok := err.(*internal.FirebaseError)
	if ok {
		return fe.Response
	}

	return nil
}

func hasPlatformErrorCode(err error, code internal.ErrorCode) bool {
	fe, ok := err.(*internal.FirebaseError)
	return ok && fe.ErrorCode == code
}
