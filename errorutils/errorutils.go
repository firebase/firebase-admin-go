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
package errorutils

import (
	"net/http"

	"firebase.google.com/go/v4/internal"
)

// ErrorCode represents the platform-wide error codes that can be raised by Firebase Admin SDK APIs.
type ErrorCode string

const (
	// InvalidArgument indicates the request was invalid or malformed.
	InvalidArgument ErrorCode = "INVALID_ARGUMENT"

	// FailedPrecondition indicates the request could not be executed in the current system state.
	FailedPrecondition ErrorCode = "FAILED_PRECONDITION"

	// OutOfRange indicates an invalid range was specified by the client.
	OutOfRange ErrorCode = "OUT_OF_RANGE"

	// Unauthenticated indicates the request lacks valid authentication credentials.
	Unauthenticated ErrorCode = "UNAUTHENTICATED"

	// PermissionDenied indicates the client does not have sufficient permission.
	PermissionDenied ErrorCode = "PERMISSION_DENIED"

	// NotFound indicates the specified resource was not found.
	NotFound ErrorCode = "NOT_FOUND"

	// Conflict indicates a conflict occurred (e.g., concurrent modification).
	Conflict ErrorCode = "CONFLICT"

	// Aborted indicates the operation was aborted (e.g., transaction conflict).
	Aborted ErrorCode = "ABORTED"

	// AlreadyExists indicates the resource already exists.
	AlreadyExists ErrorCode = "ALREADY_EXISTS"

	// ResourceExhausted indicates a quota or rate limit was exceeded.
	ResourceExhausted ErrorCode = "RESOURCE_EXHAUSTED"

	// Cancelled indicates the operation was cancelled by the client.
	Cancelled ErrorCode = "CANCELLED"

	// DataLoss indicates unrecoverable data loss or corruption.
	DataLoss ErrorCode = "DATA_LOSS"

	// Unknown indicates an unknown server error.
	Unknown ErrorCode = "UNKNOWN"

	// Internal indicates an internal server error.
	Internal ErrorCode = "INTERNAL"

	// Unavailable indicates the service is currently unavailable.
	Unavailable ErrorCode = "UNAVAILABLE"

	// DeadlineExceeded indicates the request exceeded the deadline.
	DeadlineExceeded ErrorCode = "DEADLINE_EXCEEDED"
)

// MessagingErrorCode represents FCM-specific error codes.
type MessagingErrorCode string

const (
	// MessagingAPNSAuthError indicates an error with APNS authentication.
	MessagingAPNSAuthError MessagingErrorCode = "APNS_AUTH_ERROR"

	// MessagingInternal indicates an internal messaging service error.
	MessagingInternal MessagingErrorCode = "INTERNAL"

	// MessagingThirdPartyAuthError indicates an error with third-party authentication.
	MessagingThirdPartyAuthError MessagingErrorCode = "THIRD_PARTY_AUTH_ERROR"

	// MessagingInvalidArgument indicates an invalid messaging argument.
	MessagingInvalidArgument MessagingErrorCode = "INVALID_ARGUMENT"

	// MessagingQuotaExceeded indicates the messaging quota was exceeded.
	MessagingQuotaExceeded MessagingErrorCode = "QUOTA_EXCEEDED"

	// MessagingSenderIDMismatch indicates a sender ID mismatch.
	MessagingSenderIDMismatch MessagingErrorCode = "SENDER_ID_MISMATCH"

	// MessagingUnregistered indicates the device token is no longer valid.
	MessagingUnregistered MessagingErrorCode = "UNREGISTERED"

	// MessagingUnavailable indicates the messaging service is unavailable.
	MessagingUnavailable MessagingErrorCode = "UNAVAILABLE"
)

// AuthErrorCode represents Auth-specific error codes.
type AuthErrorCode string

const (
	// AuthConfigurationNotFound indicates the configuration was not found.
	AuthConfigurationNotFound AuthErrorCode = "CONFIGURATION_NOT_FOUND"

	// AuthEmailAlreadyExists indicates the email address is already in use.
	AuthEmailAlreadyExists AuthErrorCode = "EMAIL_ALREADY_EXISTS"

	// AuthEmailNotFound indicates no user record found for the email.
	AuthEmailNotFound AuthErrorCode = "EMAIL_NOT_FOUND"

	// AuthInvalidDynamicLinkDomain indicates an invalid dynamic link domain.
	AuthInvalidDynamicLinkDomain AuthErrorCode = "INVALID_DYNAMIC_LINK_DOMAIN"

	// AuthInvalidEmail indicates the email address is invalid.
	AuthInvalidEmail AuthErrorCode = "INVALID_EMAIL"

	// AuthInvalidPageToken indicates the page token is invalid.
	AuthInvalidPageToken AuthErrorCode = "INVALID_PAGE_TOKEN"

	// AuthPhoneNumberAlreadyExists indicates the phone number is already in use.
	AuthPhoneNumberAlreadyExists AuthErrorCode = "PHONE_NUMBER_ALREADY_EXISTS"

	// AuthProjectNotFound indicates the project was not found.
	AuthProjectNotFound AuthErrorCode = "PROJECT_NOT_FOUND"

	// AuthUIDAlreadyExists indicates the UID is already in use.
	AuthUIDAlreadyExists AuthErrorCode = "UID_ALREADY_EXISTS"

	// AuthUnauthorizedContinueURL indicates an unauthorized continue URL.
	AuthUnauthorizedContinueURL AuthErrorCode = "UNAUTHORIZED_CONTINUE_URL"

	// AuthUserNotFound indicates no user record found for the identifier.
	AuthUserNotFound AuthErrorCode = "USER_NOT_FOUND"

	// AuthIDTokenExpired indicates the ID token has expired.
	AuthIDTokenExpired AuthErrorCode = "ID_TOKEN_EXPIRED"

	// AuthIDTokenInvalid indicates the ID token is invalid.
	AuthIDTokenInvalid AuthErrorCode = "ID_TOKEN_INVALID"

	// AuthIDTokenRevoked indicates the ID token has been revoked.
	AuthIDTokenRevoked AuthErrorCode = "ID_TOKEN_REVOKED"

	// AuthSessionCookieExpired indicates the session cookie has expired.
	AuthSessionCookieExpired AuthErrorCode = "SESSION_COOKIE_EXPIRED"

	// AuthSessionCookieInvalid indicates the session cookie is invalid.
	AuthSessionCookieInvalid AuthErrorCode = "SESSION_COOKIE_INVALID"

	// AuthSessionCookieRevoked indicates the session cookie has been revoked.
	AuthSessionCookieRevoked AuthErrorCode = "SESSION_COOKIE_REVOKED"

	// AuthUserDisabled indicates the user account has been disabled.
	AuthUserDisabled AuthErrorCode = "USER_DISABLED"

	// AuthTenantIDMismatch indicates a tenant ID mismatch.
	AuthTenantIDMismatch AuthErrorCode = "TENANT_ID_MISMATCH"

	// AuthCertificateFetchFailed indicates a failure to fetch certificates.
	AuthCertificateFetchFailed AuthErrorCode = "CERTIFICATE_FETCH_FAILED"

	// AuthInsufficientPermission indicates insufficient permission.
	AuthInsufficientPermission AuthErrorCode = "INSUFFICIENT_PERMISSION"

	// AuthTenantNotFound indicates the tenant was not found.
	AuthTenantNotFound AuthErrorCode = "TENANT_NOT_FOUND"
)

// FirebaseError represents an error returned by a Firebase service.
// It provides detailed information about platform-wide errors and service-specific error codes.
type FirebaseError struct {
	// ErrorCode is the platform-wide error code (e.g., INVALID_ARGUMENT, NOT_FOUND).
	ErrorCode ErrorCode

	// Message is the human-readable error message.
	Message string

	// Response is the HTTP response that caused the error (buffered copy).
	// This may be nil for errors not caused by HTTP responses.
	Response *http.Response

	// MessagingErrorCode is the FCM-specific error code, if applicable.
	// Empty string if this is not a messaging error.
	MessagingErrorCode MessagingErrorCode

	// AuthErrorCode is the Auth-specific error code, if applicable.
	// Empty string if this is not an auth error.
	AuthErrorCode AuthErrorCode
}

// Error implements the error interface.
func (e *FirebaseError) Error() string {
	return e.Message
}

// AsFirebaseError converts an error to a *FirebaseError if it is a Firebase error.
// Returns nil if the error is not a Firebase error.
//
// This function allows SDK users to access detailed error information including
// platform error codes, service-specific error codes, and HTTP response details.
//
// Example usage:
//
//	_, err := client.Send(ctx, message)
//	if fbErr := errorutils.AsFirebaseError(err); fbErr != nil {
//	    switch fbErr.ErrorCode {
//	    case errorutils.NotFound:
//	        // Handle not found error
//	    case errorutils.InvalidArgument:
//	        // Handle invalid argument error
//	    }
//
//	    // Access messaging-specific error codes
//	    if fbErr.MessagingErrorCode == errorutils.MessagingUnregistered {
//	        // Remove device token from database
//	    }
//	}
func AsFirebaseError(err error) *FirebaseError {
	fe, ok := err.(*internal.FirebaseError)
	if !ok {
		return nil
	}

	pubErr := &FirebaseError{
		ErrorCode: ErrorCode(fe.ErrorCode),
		Message:   fe.String,
		Response:  fe.Response,
	}

	// Extract messaging-specific error code if present
	if msgCode, ok := fe.Ext["messagingErrorCode"].(string); ok {
		pubErr.MessagingErrorCode = MessagingErrorCode(msgCode)
	}

	// Extract auth-specific error code if present
	if authCode, ok := fe.Ext["authErrorCode"].(string); ok {
		pubErr.AuthErrorCode = AuthErrorCode(authCode)
	}

	return pubErr
}

// IsInvalidArgument checks if the given error was due to an invalid client argument.
func IsInvalidArgument(err error) bool {
	return internal.HasPlatformErrorCode(err, internal.InvalidArgument)
}

// IsFailedPrecondition checks if the given error was because a request could not be executed
// in the current system state, such as deleting a non-empty directory.
func IsFailedPrecondition(err error) bool {
	return internal.HasPlatformErrorCode(err, internal.FailedPrecondition)
}

// IsOutOfRange checks if the given error due to an invalid range specified by the client.
func IsOutOfRange(err error) bool {
	return internal.HasPlatformErrorCode(err, internal.OutOfRange)
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
//
// This represents an HTTP 409 Conflict status code, without additional information to distinguish
// between ABORTED or ALREADY_EXISTS error conditions.
func IsConflict(err error) bool {
	return internal.HasPlatformErrorCode(err, internal.Conflict)
}

// IsAborted checks if the given error was due to a concurrency conflict, such as a
// read-modify-write conflict.
func IsAborted(err error) bool {
	return internal.HasPlatformErrorCode(err, internal.Aborted)
}

// IsAlreadyExists checks if the given error was because a resource that a client tried to create
// already exists.
func IsAlreadyExists(err error) bool {
	return internal.HasPlatformErrorCode(err, internal.AlreadyExists)
}

// IsResourceExhausted checks if the given error was caused by either running out of a quota or
// reaching a rate limit.
func IsResourceExhausted(err error) bool {
	return internal.HasPlatformErrorCode(err, internal.ResourceExhausted)
}

// IsCancelled checks if the given error was due to the client cancelling a request.
func IsCancelled(err error) bool {
	return internal.HasPlatformErrorCode(err, internal.Cancelled)
}

// IsDataLoss checks if the given error was due to an unrecoverable data loss or corruption.
//
// The client should report such errors to the end user.
func IsDataLoss(err error) bool {
	return internal.HasPlatformErrorCode(err, internal.DataLoss)
}

// IsUnknown checks if the given error was cuased by an unknown server error.
//
// This typically indicates a server bug.
func IsUnknown(err error) bool {
	return internal.HasPlatformErrorCode(err, internal.Unknown)
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

// IsDeadlineExceeded checks if the given error was due a request exceeding a deadline.
//
// This will happen only if the caller sets a deadline that is shorter than the method's default
// deadline (i.e. requested deadline is not enough for the server to process the request) and the
// request did not finish within the deadline.
func IsDeadlineExceeded(err error) bool {
	return internal.HasPlatformErrorCode(err, internal.DeadlineExceeded)
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
