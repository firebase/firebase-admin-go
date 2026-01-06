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
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"firebase.google.com/go/v4/errorutils"
	"firebase.google.com/go/v4/internal"
)

func TestGetQuotaFailureEdgeCases(t *testing.T) {
	tests := []struct {
		name string
		err  *errorutils.FirebaseError
	}{
		{"nil error", nil},
		{"no ext", &errorutils.FirebaseError{ErrorCode: internal.Unknown, Message: "test"}},
		{"empty ext", &errorutils.FirebaseError{ErrorCode: internal.Unknown, Ext: map[string]interface{}{}}},
		{"wrong type", &errorutils.FirebaseError{ErrorCode: internal.Unknown, Ext: map[string]interface{}{"quotaFailure": "string"}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if qf := GetQuotaFailure(tc.err); qf != nil {
				t.Errorf("GetQuotaFailure() = %v; want nil", qf)
			}
		})
	}
}

// TestQuotaFailureParsing verifies all QuotaViolation fields are parsed correctly from FCM responses.
func TestQuotaFailureParsing(t *testing.T) {
	resp := `{
		"error": {
			"status": "RESOURCE_EXHAUSTED",
			"message": "Quota exceeded",
			"details": [
				{"@type": "type.googleapis.com/google.firebase.fcm.v1.FcmError", "errorCode": "QUOTA_EXCEEDED"},
				{
					"@type": "type.googleapis.com/google.rpc.QuotaFailure",
					"violations": [
						{
							"subject": "project:test-project",
							"description": "Device message rate exceeded",
							"api_service": "firebasecloudmessaging.googleapis.com",
							"quota_metric": "firebasecloudmessaging.googleapis.com/device_messages",
							"quota_id": "DeviceMessagesPerMinute",
							"quota_dimensions": {"device_token": "abc123"},
							"quota_value": 100,
							"future_quota_value": 200
						},
						{"subject": "device:token123", "description": "Second violation"}
					]
				}
			]
		}
	}`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(resp))
	}))
	defer ts.Close()

	client, err := NewClient(context.Background(), testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.fcmEndpoint = ts.URL
	client.fcmClient.httpClient.RetryConfig = nil

	_, sendErr := client.Send(context.Background(), &Message{Topic: "topic"})
	if sendErr == nil {
		t.Fatal("Send() = nil; want error")
	}

	// Verify error type checks work
	if !IsQuotaExceeded(sendErr) {
		t.Error("IsQuotaExceeded() = false; want true")
	}
	if !IsMessageRateExceeded(sendErr) { // deprecated alias
		t.Error("IsMessageRateExceeded() = false; want true")
	}

	fe := sendErr.(*errorutils.FirebaseError)
	qf := GetQuotaFailure(fe)
	if qf == nil {
		t.Fatal("GetQuotaFailure() = nil; want non-nil")
	}
	if len(qf.Violations) != 2 {
		t.Fatalf("len(Violations) = %d; want 2", len(qf.Violations))
	}

	// Verify all fields on first violation
	v := qf.Violations[0]
	if v.Subject != "project:test-project" {
		t.Errorf("Subject = %q; want %q", v.Subject, "project:test-project")
	}
	if v.APIService != "firebasecloudmessaging.googleapis.com" {
		t.Errorf("APIService = %q; want %q", v.APIService, "firebasecloudmessaging.googleapis.com")
	}
	if v.QuotaMetric != "firebasecloudmessaging.googleapis.com/device_messages" {
		t.Errorf("QuotaMetric = %q; want correct value", v.QuotaMetric)
	}
	if v.QuotaID != "DeviceMessagesPerMinute" {
		t.Errorf("QuotaID = %q; want %q", v.QuotaID, "DeviceMessagesPerMinute")
	}
	if v.QuotaDimensions == nil || v.QuotaDimensions["device_token"] != "abc123" {
		t.Errorf("QuotaDimensions = %v; want map with device_token=abc123", v.QuotaDimensions)
	}
	if v.QuotaValue != 100 || v.FutureQuotaValue != 200 {
		t.Errorf("QuotaValue=%d, FutureQuotaValue=%d; want 100, 200", v.QuotaValue, v.FutureQuotaValue)
	}

	// Verify second violation
	if qf.Violations[1].Subject != "device:token123" {
		t.Errorf("Violations[1].Subject = %q; want %q", qf.Violations[1].Subject, "device:token123")
	}
}

// TestDeprecatedErrorFunctions verifies deprecated functions still work for backwards compatibility.
func TestDeprecatedErrorFunctions(t *testing.T) {
	tests := []struct {
		name       string
		httpStatus int
		resp       string
		deprecated func(error) bool
		current    func(error) bool
	}{
		{
			name:       "IsInvalidAPNSCredentials",
			httpStatus: http.StatusUnauthorized,
			resp:       `{"error": {"status": "UNAUTHENTICATED", "message": "test", "details": [{"@type": "type.googleapis.com/google.firebase.fcm.v1.FcmError", "errorCode": "THIRD_PARTY_AUTH_ERROR"}]}}`,
			deprecated: IsInvalidAPNSCredentials,
			current:    IsThirdPartyAuthError,
		},
		{
			name:       "IsMismatchedCredential",
			httpStatus: http.StatusForbidden,
			resp:       `{"error": {"status": "PERMISSION_DENIED", "message": "test", "details": [{"@type": "type.googleapis.com/google.firebase.fcm.v1.FcmError", "errorCode": "SENDER_ID_MISMATCH"}]}}`,
			deprecated: IsMismatchedCredential,
			current:    IsSenderIDMismatch,
		},
		{
			name:       "IsRegistrationTokenNotRegistered",
			httpStatus: http.StatusNotFound,
			resp:       `{"error": {"status": "NOT_FOUND", "message": "test", "details": [{"@type": "type.googleapis.com/google.firebase.fcm.v1.FcmError", "errorCode": "UNREGISTERED"}]}}`,
			deprecated: IsRegistrationTokenNotRegistered,
			current:    IsUnregistered,
		},
		{
			name:       "IsServerUnavailable",
			httpStatus: http.StatusServiceUnavailable,
			resp:       `{"error": {"status": "UNAVAILABLE", "message": "test", "details": [{"@type": "type.googleapis.com/google.firebase.fcm.v1.FcmError", "errorCode": "UNAVAILABLE"}]}}`,
			deprecated: IsServerUnavailable,
			current:    IsUnavailable,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.httpStatus)
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(tc.resp))
			}))
			defer ts.Close()

			client, _ := NewClient(context.Background(), testMessagingConfig)
			client.fcmEndpoint = ts.URL
			client.fcmClient.httpClient.RetryConfig = nil

			_, err := client.Send(context.Background(), &Message{Topic: "topic"})
			if !tc.deprecated(err) {
				t.Errorf("%s() = false; want true", tc.name)
			}
			if !tc.current(err) {
				t.Errorf("current function = false; want true")
			}
		})
	}
}

// TestDeprecatedFunctionsAlwaysFalse verifies IsTooManyTopics and IsUnknown always return false.
func TestDeprecatedFunctionsAlwaysFalse(t *testing.T) {
	if IsTooManyTopics(nil) {
		t.Error("IsTooManyTopics(nil) = true; want false")
	}
	if IsUnknown(nil) {
		t.Error("IsUnknown(nil) = true; want false")
	}
}
