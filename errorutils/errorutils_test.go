// Copyright 2025 Google Inc. All Rights Reserved.
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
	"net/http"
	"testing"

	"firebase.google.com/go/v4/internal"
)

func TestAsFirebaseError_Success(t *testing.T) {
	internalErr := &internal.FirebaseError{
		ErrorCode: internal.InvalidArgument,
		String:    "test error message",
		Response: &http.Response{
			StatusCode: 400,
		},
		Ext: map[string]interface{}{
			"messagingErrorCode": "UNREGISTERED",
		},
	}

	fbErr := AsFirebaseError(internalErr)
	if fbErr == nil {
		t.Fatal("AsFirebaseError() returned nil for valid FirebaseError")
	}

	if fbErr.ErrorCode != InvalidArgument {
		t.Errorf("ErrorCode = %v, want %v", fbErr.ErrorCode, InvalidArgument)
	}

	if fbErr.Message != "test error message" {
		t.Errorf("Message = %q, want %q", fbErr.Message, "test error message")
	}

	if fbErr.Response == nil || fbErr.Response.StatusCode != 400 {
		t.Error("Response not properly set")
	}

	if fbErr.MessagingErrorCode != MessagingUnregistered {
		t.Errorf("MessagingErrorCode = %v, want %v", fbErr.MessagingErrorCode, MessagingUnregistered)
	}

	if fbErr.AuthErrorCode != "" {
		t.Errorf("AuthErrorCode = %v, want empty string", fbErr.AuthErrorCode)
	}
}

func TestAsFirebaseError_AuthError(t *testing.T) {
	internalErr := &internal.FirebaseError{
		ErrorCode: internal.NotFound,
		String:    "user not found",
		Ext: map[string]interface{}{
			"authErrorCode": "USER_NOT_FOUND",
		},
	}

	fbErr := AsFirebaseError(internalErr)
	if fbErr == nil {
		t.Fatal("AsFirebaseError() returned nil for valid FirebaseError")
	}

	if fbErr.ErrorCode != NotFound {
		t.Errorf("ErrorCode = %v, want %v", fbErr.ErrorCode, NotFound)
	}

	if fbErr.AuthErrorCode != AuthUserNotFound {
		t.Errorf("AuthErrorCode = %v, want %v", fbErr.AuthErrorCode, AuthUserNotFound)
	}

	if fbErr.MessagingErrorCode != "" {
		t.Errorf("MessagingErrorCode = %v, want empty string", fbErr.MessagingErrorCode)
	}
}

func TestAsFirebaseError_BothErrorCodes(t *testing.T) {
	// This shouldn't happen in practice, but test it anyway
	internalErr := &internal.FirebaseError{
		ErrorCode: internal.Internal,
		String:    "internal error",
		Ext: map[string]interface{}{
			"messagingErrorCode": "INTERNAL",
			"authErrorCode":      "INSUFFICIENT_PERMISSION",
		},
	}

	fbErr := AsFirebaseError(internalErr)
	if fbErr == nil {
		t.Fatal("AsFirebaseError() returned nil for valid FirebaseError")
	}

	if fbErr.MessagingErrorCode != MessagingInternal {
		t.Errorf("MessagingErrorCode = %v, want %v", fbErr.MessagingErrorCode, MessagingInternal)
	}

	if fbErr.AuthErrorCode != AuthInsufficientPermission {
		t.Errorf("AuthErrorCode = %v, want %v", fbErr.AuthErrorCode, AuthInsufficientPermission)
	}
}

func TestAsFirebaseError_NoServiceErrorCode(t *testing.T) {
	internalErr := &internal.FirebaseError{
		ErrorCode: internal.Unavailable,
		String:    "service unavailable",
		Ext:       map[string]interface{}{},
	}

	fbErr := AsFirebaseError(internalErr)
	if fbErr == nil {
		t.Fatal("AsFirebaseError() returned nil for valid FirebaseError")
	}

	if fbErr.ErrorCode != Unavailable {
		t.Errorf("ErrorCode = %v, want %v", fbErr.ErrorCode, Unavailable)
	}

	if fbErr.MessagingErrorCode != "" {
		t.Errorf("MessagingErrorCode = %v, want empty string", fbErr.MessagingErrorCode)
	}

	if fbErr.AuthErrorCode != "" {
		t.Errorf("AuthErrorCode = %v, want empty string", fbErr.AuthErrorCode)
	}
}

func TestAsFirebaseError_NonFirebaseError(t *testing.T) {
	// Test with a regular error
	regularErr := &testError{msg: "regular error"}

	fbErr := AsFirebaseError(regularErr)
	if fbErr != nil {
		t.Errorf("AsFirebaseError() = %v, want nil for non-Firebase error", fbErr)
	}

	// Test with nil
	fbErr = AsFirebaseError(nil)
	if fbErr != nil {
		t.Errorf("AsFirebaseError(nil) = %v, want nil", fbErr)
	}
}

func TestFirebaseError_Error(t *testing.T) {
	fbErr := &FirebaseError{
		ErrorCode: InvalidArgument,
		Message:   "test error message",
	}

	if fbErr.Error() != "test error message" {
		t.Errorf("Error() = %q, want %q", fbErr.Error(), "test error message")
	}
}

func TestErrorCodeConstants(t *testing.T) {
	// Verify that platform error codes match internal error codes
	tests := []struct {
		public   ErrorCode
		internal internal.ErrorCode
	}{
		{InvalidArgument, internal.InvalidArgument},
		{FailedPrecondition, internal.FailedPrecondition},
		{OutOfRange, internal.OutOfRange},
		{Unauthenticated, internal.Unauthenticated},
		{PermissionDenied, internal.PermissionDenied},
		{NotFound, internal.NotFound},
		{Conflict, internal.Conflict},
		{Aborted, internal.Aborted},
		{AlreadyExists, internal.AlreadyExists},
		{ResourceExhausted, internal.ResourceExhausted},
		{Cancelled, internal.Cancelled},
		{DataLoss, internal.DataLoss},
		{Unknown, internal.Unknown},
		{Internal, internal.Internal},
		{Unavailable, internal.Unavailable},
		{DeadlineExceeded, internal.DeadlineExceeded},
	}

	for _, tt := range tests {
		if string(tt.public) != string(tt.internal) {
			t.Errorf("ErrorCode mismatch: public=%q, internal=%q", tt.public, tt.internal)
		}
	}
}

func TestMessagingErrorCodeValues(t *testing.T) {
	// Verify messaging error code values are as expected
	tests := []struct {
		code     MessagingErrorCode
		expected string
	}{
		{MessagingAPNSAuthError, "APNS_AUTH_ERROR"},
		{MessagingInternal, "INTERNAL"},
		{MessagingThirdPartyAuthError, "THIRD_PARTY_AUTH_ERROR"},
		{MessagingInvalidArgument, "INVALID_ARGUMENT"},
		{MessagingQuotaExceeded, "QUOTA_EXCEEDED"},
		{MessagingSenderIDMismatch, "SENDER_ID_MISMATCH"},
		{MessagingUnregistered, "UNREGISTERED"},
		{MessagingUnavailable, "UNAVAILABLE"},
	}

	for _, tt := range tests {
		if string(tt.code) != tt.expected {
			t.Errorf("MessagingErrorCode value mismatch: got %q, want %q", tt.code, tt.expected)
		}
	}
}

func TestAuthErrorCodeValues(t *testing.T) {
	// Verify auth error code values are as expected
	tests := []struct {
		code     AuthErrorCode
		expected string
	}{
		{AuthConfigurationNotFound, "CONFIGURATION_NOT_FOUND"},
		{AuthEmailAlreadyExists, "EMAIL_ALREADY_EXISTS"},
		{AuthEmailNotFound, "EMAIL_NOT_FOUND"},
		{AuthInvalidDynamicLinkDomain, "INVALID_DYNAMIC_LINK_DOMAIN"},
		{AuthInvalidEmail, "INVALID_EMAIL"},
		{AuthInvalidPageToken, "INVALID_PAGE_TOKEN"},
		{AuthPhoneNumberAlreadyExists, "PHONE_NUMBER_ALREADY_EXISTS"},
		{AuthProjectNotFound, "PROJECT_NOT_FOUND"},
		{AuthUIDAlreadyExists, "UID_ALREADY_EXISTS"},
		{AuthUnauthorizedContinueURL, "UNAUTHORIZED_CONTINUE_URL"},
		{AuthUserNotFound, "USER_NOT_FOUND"},
		{AuthIDTokenExpired, "ID_TOKEN_EXPIRED"},
		{AuthIDTokenInvalid, "ID_TOKEN_INVALID"},
		{AuthIDTokenRevoked, "ID_TOKEN_REVOKED"},
		{AuthSessionCookieExpired, "SESSION_COOKIE_EXPIRED"},
		{AuthSessionCookieInvalid, "SESSION_COOKIE_INVALID"},
		{AuthSessionCookieRevoked, "SESSION_COOKIE_REVOKED"},
		{AuthUserDisabled, "USER_DISABLED"},
		{AuthTenantIDMismatch, "TENANT_ID_MISMATCH"},
		{AuthCertificateFetchFailed, "CERTIFICATE_FETCH_FAILED"},
		{AuthInsufficientPermission, "INSUFFICIENT_PERMISSION"},
		{AuthTenantNotFound, "TENANT_NOT_FOUND"},
	}

	for _, tt := range tests {
		if string(tt.code) != tt.expected {
			t.Errorf("AuthErrorCode value mismatch: got %q, want %q", tt.code, tt.expected)
		}
	}
}

// testError is a simple error type for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
