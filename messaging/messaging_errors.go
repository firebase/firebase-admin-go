// Copyright 2018 Google LLC All Rights Reserved.
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

package messaging

import (
	"encoding/json"

	"firebase.google.com/go/v4/errorutils"
	"firebase.google.com/go/v4/internal"
)

// FCM error codes
const (
	apnsAuthError       = "APNS_AUTH_ERROR"
	internalError       = "INTERNAL"
	thirdPartyAuthError = "THIRD_PARTY_AUTH_ERROR"
	invalidArgument     = "INVALID_ARGUMENT"
	quotaExceeded       = "QUOTA_EXCEEDED"
	senderIDMismatch    = "SENDER_ID_MISMATCH"
	unregistered        = "UNREGISTERED"
	unavailable         = "UNAVAILABLE"
)

// QuotaViolation describes a single quota violation, identifying which quota
// was exceeded.
// See https://docs.cloud.google.com/tasks/docs/reference/rpc/google.rpc#google.rpc.QuotaFailure.Violation
// for more information on the google.rpc.QuotaFailure.Violation type.
type QuotaViolation struct {
	// Subject is the subject on which the quota check failed.
	// For example, "clientip:<ip address>" or "project:<project id>".
	Subject string
	// Description explains how the quota check failed.
	Description string
	// APIService is the API service from which the QuotaFailure originates.
	APIService string
	// QuotaMetric is the metric of the violated quota (e.g., "compute.googleapis.com/cpus").
	QuotaMetric string
	// QuotaID is the unique identifier of a quota (e.g., "CPUS-per-project-region").
	QuotaID string
	// QuotaDimensions contains the dimensions of the violated quota.
	QuotaDimensions map[string]string
	// QuotaValue is the enforced quota value at the time of the failure.
	QuotaValue int64
	// FutureQuotaValue is the new quota value being rolled out, if a rollout is in progress.
	FutureQuotaValue int64
}

// QuotaFailure contains information about quota violations from FCM.
// This is returned when a rate limit is exceeded (device, topic, or overall).
type QuotaFailure struct {
	Violations []*QuotaViolation
}

// IsInternal checks if the given error was due to an internal server error.
func IsInternal(err error) bool {
	return hasMessagingErrorCode(err, internalError)
}

// IsInvalidAPNSCredentials checks if the given error was due to invalid APNS certificate or auth
// key.
//
// Deprecated: Use IsThirdPartyAuthError().
func IsInvalidAPNSCredentials(err error) bool {
	return IsThirdPartyAuthError(err)
}

// IsThirdPartyAuthError checks if the given error was due to invalid APNS certificate or auth
// key.
func IsThirdPartyAuthError(err error) bool {
	return hasMessagingErrorCode(err, thirdPartyAuthError) || hasMessagingErrorCode(err, apnsAuthError)
}

// IsInvalidArgument checks if the given error was due to an invalid argument in the request.
func IsInvalidArgument(err error) bool {
	return hasMessagingErrorCode(err, invalidArgument)
}

// IsMessageRateExceeded checks if the given error was due to the client exceeding a quota.
//
// Deprecated: Use IsQuotaExceeded().
func IsMessageRateExceeded(err error) bool {
	return IsQuotaExceeded(err)
}

// IsQuotaExceeded checks if the given error was due to the client exceeding a quota.
func IsQuotaExceeded(err error) bool {
	return hasMessagingErrorCode(err, quotaExceeded)
}


// IsMismatchedCredential checks if the given error was due to an invalid credential or permission
// error.
//
// Deprecated: Use IsSenderIDMismatch().
func IsMismatchedCredential(err error) bool {
	return IsSenderIDMismatch(err)
}

// IsSenderIDMismatch checks if the given error was due to an invalid credential or permission
// error.
func IsSenderIDMismatch(err error) bool {
	return hasMessagingErrorCode(err, senderIDMismatch)
}

// IsRegistrationTokenNotRegistered checks if the given error was due to a registration token that
// became invalid.
//
// Deprecated: Use IsUnregistered().
func IsRegistrationTokenNotRegistered(err error) bool {
	return IsUnregistered(err)
}

// IsUnregistered checks if the given error was due to a registration token that
// became invalid.
func IsUnregistered(err error) bool {
	return hasMessagingErrorCode(err, unregistered)
}

// IsServerUnavailable checks if the given error was due to the backend server being temporarily
// unavailable.
//
// Deprecated: Use IsUnavailable().
func IsServerUnavailable(err error) bool {
	return IsUnavailable(err)
}

// IsUnavailable checks if the given error was due to the backend server being temporarily
// unavailable.
func IsUnavailable(err error) bool {
	return hasMessagingErrorCode(err, unavailable)
}

// IsTooManyTopics checks if the given error was due to the client exceeding the allowed number
// of topics.
//
// Deprecated: Always returns false.
func IsTooManyTopics(err error) bool {
	return false
}

// IsUnknown checks if the given error was due to unknown error returned by the backend server.
//
// Deprecated: Always returns false.
func IsUnknown(err error) bool {
	return false
}

type fcmErrorResponse struct {
	Error struct {
		Details []fcmErrorDetail `json:"details"`
	} `json:"error"`
}

type fcmErrorDetail struct {
	Type       string `json:"@type"`
	ErrorCode  string `json:"errorCode"`
	Violations []struct {
		Subject          string            `json:"subject"`
		Description      string            `json:"description"`
		APIService       string            `json:"api_service"`
		QuotaMetric      string            `json:"quota_metric"`
		QuotaID          string            `json:"quota_id"`
		QuotaDimensions  map[string]string `json:"quota_dimensions"`
		QuotaValue       int64             `json:"quota_value"`
		FutureQuotaValue int64             `json:"future_quota_value"`
	} `json:"violations"`
}

func handleFCMError(resp *internal.Response) error {
	base := internal.NewFirebaseErrorOnePlatform(resp)
	var fe fcmErrorResponse
	json.Unmarshal(resp.Body, &fe) // ignore any json parse errors at this level

	// FCM error responses include a "details" array with typed extensions.
	// See https://firebase.google.com/docs/reference/fcm/rest/v1/ErrorCode
	for _, d := range fe.Error.Details {
		// FcmError extension contains the FCM-specific error code.
		// See https://firebase.google.com/docs/reference/fcm/rest/v1/FcmError
		if d.Type == "type.googleapis.com/google.firebase.fcm.v1.FcmError" {
			base.Ext["messagingErrorCode"] = d.ErrorCode
		}
		// QuotaFailure extension is returned when QUOTA_EXCEEDED error occurs.
		// See https://firebase.google.com/docs/reference/fcm/rest/v1/ErrorCode
		// "An extension of type google.rpc.QuotaFailure is returned to specify which quota was exceeded."
		if d.Type == "type.googleapis.com/google.rpc.QuotaFailure" && len(d.Violations) > 0 {
			violations := make([]*QuotaViolation, len(d.Violations))
			for i, v := range d.Violations {
				violations[i] = &QuotaViolation{
					Subject:          v.Subject,
					Description:      v.Description,
					APIService:       v.APIService,
					QuotaMetric:      v.QuotaMetric,
					QuotaID:          v.QuotaID,
					QuotaDimensions:  v.QuotaDimensions,
					QuotaValue:       v.QuotaValue,
					FutureQuotaValue: v.FutureQuotaValue,
				}
			}
			base.Ext["quotaFailure"] = &QuotaFailure{Violations: violations}
		}
	}

	return base
}

func hasMessagingErrorCode(err error, code string) bool {
	fe, ok := err.(*internal.FirebaseError)
	if !ok {
		return false
	}

	got, ok := fe.Ext["messagingErrorCode"]
	return ok && got == code
}
