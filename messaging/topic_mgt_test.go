// Copyright 2019 Google Inc. All Rights Reserved.
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
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"firebase.google.com/go/v4/app" // Import app package
	"firebase.google.com/go/v4/errorutils"
	"firebase.google.com/go/v4/internal"
	"google.golang.org/api/option" // For app options
)

// testMessagingProjectID is defined in messaging_test.go, assuming it's accessible
// or we redefine it here if necessary. For now, assume it's "test-project".

// Helper to create a new app.App for Messaging Topic Management tests
func newTestTopicApp(ctx context.Context) *app.App {
	opts := []option.ClientOption{
		option.WithTokenSource(&internal.MockTokenSource{AccessToken: "test-token"}),
		option.WithScopes(internal.FirebaseScopes...),
	}
	appInstance, err := app.New(ctx, &app.Config{ProjectID: testMessagingProjectID}, opts...)
	if err != nil {
		log.Fatalf("Error creating test app for Messaging (topic_mgt): %v", err)
	}
	return appInstance
}


func TestSubscribe(t *testing.T) {
	var tr *http.Request
	var b []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tr = r
		b, _ = ioutil.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{\"results\": [{}, {\"error\": \"error_reason\"}]}"))
	}))
	defer ts.Close()

	ctx := context.Background()
	appInstance := newTestTopicApp(ctx)
	client, err := NewClient(ctx, appInstance) // NewClient now takes *app.App
	if err != nil {
		t.Fatal(err)
	}
	// The iidClient within the messaging client handles its own endpoint.
	// We need to ensure the test server URL is used by the internal iidClient.
	// This requires the iidClient to be configurable or to use the app's http client,
	// which can be a test client.
	// For simplicity in this refactor, if iidClient is not using app's http client directly for its endpoint,
	// we'd have to modify it or accept this test might not hit the mock for iid part.
	// However, `newIIDClient` (in topic_mgt.go) takes an `hc *http.Client`.
	// This `hc` comes from `appInstance.Options()` in `messaging.NewClient`.
	// So, if we want `iidClient` to talk to `ts`, `appInstance` needs an http client pointing to `ts`.
	// This is complex. A simpler mock for `iidClient` methods might be needed if direct http override is hard.

	// For now, let's assume the internal iidClient's httpClient will be correctly
	// configured if the appInstance passed to messaging.NewClient has a test http client.
	// The `newTestTopicApp` creates a generic app.
	// A more robust test setup would involve creating an app with an httptest client.
	// Let's override the iidClient's endpoint for this test to ensure it hits the mock.
	if client.iidClient != nil {
		client.iidClient.iidEndpoint = ts.URL // Override internal IID endpoint
	} else {
		t.Fatal("messaging client's internal iidClient is nil")
	}


	resp, err := client.SubscribeToTopic(ctx, []string{"id1", "id2"}, "test-topic")
	if err != nil {
		t.Fatal(err)
	}
	checkIIDRequest(t, b, tr, iidSubscribe, appInstance.SDKVersion()) // Pass sdkVersion
	checkTopicMgtResponse(t, resp)
}

func TestInvalidSubscribe(t *testing.T) {
	ctx := context.Background()
	appInstance := newTestTopicApp(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range invalidTopicMgtArgs {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := client.SubscribeToTopic(ctx, tc.tokens, tc.topic)
			if err == nil || err.Error() != tc.want {
				t.Errorf(
					"SubscribeToTopic(%s) = (%#v, %v); want = (nil, %q)", tc.name, resp, err, tc.want)
			}
		})
	}
}

func TestUnsubscribe(t *testing.T) {
	var tr *http.Request
	var b []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tr = r
		b, _ = ioutil.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{\"results\": [{}, {\"error\": \"error_reason\"}]}"))
	}))
	defer ts.Close()

	ctx := context.Background()
	appInstance := newTestTopicApp(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}
	if client.iidClient != nil {
		client.iidClient.iidEndpoint = ts.URL
	} else {
		t.Fatal("messaging client's internal iidClient is nil")
	}


	resp, err := client.UnsubscribeFromTopic(ctx, []string{"id1", "id2"}, "test-topic")
	if err != nil {
		t.Fatal(err)
	}
	checkIIDRequest(t, b, tr, iidUnsubscribe, appInstance.SDKVersion()) // Pass sdkVersion
	checkTopicMgtResponse(t, resp)
}

func TestInvalidUnsubscribe(t *testing.T) {
	ctx := context.Background()
	appInstance := newTestTopicApp(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range invalidTopicMgtArgs {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := client.UnsubscribeFromTopic(ctx, tc.tokens, tc.topic)
			if err == nil || err.Error() != tc.want {
				t.Errorf(
					"UnsubscribeFromTopic(%s) = (%#v, %v); want = (nil, %q)", tc.name, resp, err, tc.want)
			}
		})
	}
}

func TestTopicManagementError(t *testing.T) {
	var respBody string // Renamed from resp to avoid conflict
	var status int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(respBody))
	}))
	defer ts.Close()

	ctx := context.Background()
	appInstance := newTestTopicApp(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}
	if client.iidClient != nil && client.iidClient.httpClient != nil {
		client.iidClient.iidEndpoint = ts.URL
		client.iidClient.httpClient.RetryConfig = nil
	} else {
		t.Fatal("messaging client's internal iidClient or its httpClient is nil")
	}


	cases := []struct {
		name, resp, want string // field resp renamed to respStr to avoid conflict
		status           int
		check            func(err error) bool
	}{
		{
			name:   "EmptyResponse",
			resp:   "{}",
			want:   "unexpected http response with status: 500\n{}",
			status: http.StatusInternalServerError,
			check:  errorutils.IsInternal,
		},
		{
			name:   "ErrorCode",
			resp:   "{\"error\": \"INVALID_ARGUMENT\"}",
			want:   "error while calling the iid service: INVALID_ARGUMENT",
			status: http.StatusBadRequest,
			check:  errorutils.IsInvalidArgument,
		},
		{
			name:   "NotJson",
			resp:   "not json",
			want:   "unexpected http response with status: 500\nnot json",
			status: http.StatusInternalServerError,
			check:  errorutils.IsInternal,
		},
	}

	for _, tc := range cases {
		respBody = tc.resp // Set the global respBody for the handler
		status = tc.status

		tmr, err := client.SubscribeToTopic(ctx, []string{"id1"}, "topic")
		if err == nil || err.Error() != tc.want || !tc.check(err) {
			t.Errorf("SubscribeToTopic(%s) = (%#v, %v); want = (nil, %q)", tc.name, tmr, err, tc.want)
		}

		tmr, err = client.UnsubscribeFromTopic(ctx, []string{"id1"}, "topic")
		if err == nil || err.Error() != tc.want || !tc.check(err) {
			t.Errorf("UnsubscribeFromTopic(%s) = (%#v, %v); want = (nil, %q)", tc.name, tmr, err, tc.want)
		}
	}
}

func checkIIDRequest(t *testing.T, b []byte, tr *http.Request, op string, sdkVersion string) {
	var parsed map[string]interface{}
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatal(err)
	}
	want := map[string]interface{}{
		"to":                  "/topics/test-topic",
		"registration_tokens": []interface{}{"id1", "id2"},
	}
	if !reflect.DeepEqual(parsed, want) {
		t.Errorf("Body = %#v; want = %#v", parsed, want)
	}

	if tr.Method != http.MethodPost {
		t.Errorf("Method = %q; want = %q", tr.Method, http.MethodPost)
	}
	wantOp := ":" + op // Path for IID batchAdd/Remove is just ":op" relative to endpoint
	if tr.URL.Path != wantOp {
		t.Errorf("Path = %q; want = %q", tr.URL.Path, wantOp)
	}
	if h := tr.Header.Get("Authorization"); h != "Bearer test-token" {
		t.Errorf("Authorization = %q; want = %q", h, "Bearer test-token")
	}
	xGoogAPIClientHeader := internal.GetMetricsHeader(sdkVersion) // Use sdkVersion
	if h := tr.Header.Get("x-goog-api-client"); h != xGoogAPIClientHeader {
		t.Errorf("x-goog-api-client header = %q; want = %q", h, xGoogAPIClientHeader)
	}
}

func checkTopicMgtResponse(t *testing.T, resp *TopicManagementResponse) {
	if resp.SuccessCount != 1 {
		t.Errorf("SuccessCount = %d; want  = %d", resp.SuccessCount, 1)
	}
	if resp.FailureCount != 1 {
		t.Errorf("FailureCount = %d; want  = %d", resp.FailureCount, 1)
	}
	if len(resp.Errors) != 1 {
		t.Fatalf("Errors = %d; want = %d", len(resp.Errors), 1)
	}
	e := resp.Errors[0]
	if e.Index != 1 {
		t.Errorf("ErrorInfo.Index = %d; want = %d", e.Index, 1)
	}
	if e.Reason != "error_reason" {
		t.Errorf("ErrorInfo.Reason = %s; want = %s", e.Reason, "error_reason")
	}
}

var invalidTopicMgtArgs = []struct {
	name   string
	tokens []string
	topic  string
	want   string
}{
	{
		name: "NoTokensAndTopic",
		want: "no tokens specified",
	},
	{
		name:   "NoTopic",
		tokens: []string{"token1"},
		want:   "topic name not specified",
	},
	{
		name:   "InvalidTopicName",
		tokens: []string{"token1"},
		topic:  "foo*bar",
		want:   "invalid topic name: \"foo*bar\"",
	},
	{
		name:   "TooManyTokens",
		tokens: strings.Split("a"+strings.Repeat(",a", 1000), ","),
		topic:  "topic",
		want:   "tokens list must not contain more than 1000 items",
	},
	{
		name:   "EmptyToken",
		tokens: []string{"foo", ""},
		topic:  "topic",
		want:   "tokens list must not contain empty strings",
	},
}
