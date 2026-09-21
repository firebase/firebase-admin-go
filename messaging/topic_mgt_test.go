// Copyright 2019 Google LLC All Rights Reserved.
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
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	"firebase.google.com/go/v4/internal"
)

func TestSubscribe(t *testing.T) {
	var mu sync.Mutex
	var requestCount int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "id2") {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error": {"status": "INVALID_ARGUMENT", "message": "error_reason"}}`))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("{}"))
		}
	}))
	defer ts.Close()

	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.fcmEndpoint = ts.URL

	resp, err := client.SubscribeToTopic(ctx, []string{"id1", "id2"}, "test-topic")
	if err != nil {
		t.Fatal(err)
	}
	checkTopicMgtResponse(t, resp, "INVALID_ARGUMENT")
	if requestCount != 2 {
		t.Errorf("got %d requests, want 2", requestCount)
	}
}

func TestSubscribeAlreadyExists409(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"error": {"status": "ALREADY_EXISTS", "message": "Already exists"}}`))
	}))
	defer ts.Close()

	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.fcmEndpoint = ts.URL

	resp, err := client.SubscribeToTopic(ctx, []string{"id1"}, "test-topic")
	if err != nil {
		t.Fatal(err)
	}
	if resp.SuccessCount != 1 || resp.FailureCount != 0 {
		t.Errorf("resp = (%d, %d), want (1, 0)", resp.SuccessCount, resp.FailureCount)
	}
}

func TestUnsubscribe(t *testing.T) {
	var mu sync.Mutex
	var requestCount int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "id2") {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error": {"status": "INVALID_ARGUMENT", "message": "error_reason"}}`))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("{}"))
		}
	}))
	defer ts.Close()

	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.fcmEndpoint = ts.URL

	resp, err := client.UnsubscribeFromTopic(ctx, []string{"id1", "id2"}, "test-topic")
	if err != nil {
		t.Fatal(err)
	}
	checkTopicMgtResponse(t, resp, "INVALID_ARGUMENT")
	if requestCount != 2 {
		t.Errorf("got %d requests, want 2", requestCount)
	}
}

func TestUnsubscribeNotFound404(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": {"status": "NOT_FOUND", "message": "Not found"}}`))
	}))
	defer ts.Close()

	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.fcmEndpoint = ts.URL

	resp, err := client.UnsubscribeFromTopic(ctx, []string{"id1"}, "test-topic")
	if err != nil {
		t.Fatal(err)
	}
	if resp.SuccessCount != 0 || resp.FailureCount != 1 {
		t.Errorf("resp = (%d, %d), want (0, 1)", resp.SuccessCount, resp.FailureCount)
	}
	if len(resp.Errors) != 1 || resp.Errors[0].Reason != "NOT_FOUND" {
		t.Errorf("Errors[0].Reason = %q, want NOT_FOUND", resp.Errors[0].Reason)
	}
}

func TestTopicManagementFcmErrorDetails(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{
			"error": {
				"status": "NOT_FOUND",
				"details": [
					{
						"@type": "type.googleapis.com/google.firebase.fcm.v1.FcmError",
						"errorCode": "UNREGISTERED"
					}
				]
			}
		}`))
	}))
	defer ts.Close()

	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.fcmEndpoint = ts.URL

	resp, err := client.SubscribeToTopic(ctx, []string{"id1"}, "test-topic")
	if err != nil {
		t.Fatal(err)
	}
	if resp.SuccessCount != 0 || resp.FailureCount != 1 {
		t.Errorf("resp = (%d, %d), want (0, 1)", resp.SuccessCount, resp.FailureCount)
	}
	if len(resp.Errors) != 1 || resp.Errors[0].Reason != "UNREGISTERED" {
		t.Errorf("Errors[0].Reason = %q, want UNREGISTERED", resp.Errors[0].Reason)
	}
}

func TestTopicManagement500Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": null}`))
	}))
	defer ts.Close()

	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.fcmEndpoint = ts.URL

	resp, err := client.SubscribeToTopic(ctx, []string{"id1"}, "test-topic")
	if err != nil {
		t.Fatal(err)
	}
	if resp.SuccessCount != 0 || resp.FailureCount != 1 {
		t.Errorf("resp = (%d, %d), want (0, 1)", resp.SuccessCount, resp.FailureCount)
	}
	if len(resp.Errors) != 1 || resp.Errors[0].Reason != "INTERNAL" {
		t.Errorf("Errors[0].Reason = %q, want INTERNAL", resp.Errors[0].Reason)
	}
}

func TestTopicManagementNonJsonError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("not json"))
	}))
	defer ts.Close()

	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.fcmEndpoint = ts.URL

	resp, err := client.SubscribeToTopic(ctx, []string{"id1"}, "test-topic")
	if err != nil {
		t.Fatal(err)
	}
	if resp.SuccessCount != 0 || resp.FailureCount != 1 {
		t.Errorf("resp = (%d, %d), want (0, 1)", resp.SuccessCount, resp.FailureCount)
	}
	if len(resp.Errors) != 1 || resp.Errors[0].Reason != "INVALID_ARGUMENT" {
		t.Errorf("Errors[0].Reason = %q, want INVALID_ARGUMENT", resp.Errors[0].Reason)
	}
}

func TestTopicManagementContextCancelled(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{}"))
	}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client, err := NewClient(context.Background(), testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.fcmEndpoint = ts.URL

	resp, err := client.SubscribeToTopic(ctx, []string{"id1"}, "test-topic")
	if err != context.Canceled {
		t.Errorf("SubscribeToTopic() = (%#v, %v); want = (nil, %v)", resp, err, context.Canceled)
	}

	resp, err = client.UnsubscribeFromTopic(ctx, []string{"id1"}, "test-topic")
	if err != context.Canceled {
		t.Errorf("UnsubscribeFromTopic() = (%#v, %v); want = (nil, %v)", resp, err, context.Canceled)
	}
}

func TestSubscribeLegacy(t *testing.T) {
	var tr *http.Request
	var b []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tr = r
		b, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{\"results\": [{}, {\"error\": \"error_reason\"}]}"))
	}))
	defer ts.Close()

	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.iidEndpoint = ts.URL + "/v1"

	resp, err := client.SubscribeToTopicLegacy(ctx, []string{"id1", "id2"}, "test-topic")
	if err != nil {
		t.Fatal(err)
	}
	checkIIDRequest(t, b, tr, iidSubscribe)
	checkTopicMgtResponse(t, resp, "error_reason")
}

func TestUnsubscribeLegacy(t *testing.T) {
	var tr *http.Request
	var b []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tr = r
		b, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{\"results\": [{}, {\"error\": \"error_reason\"}]}"))
	}))
	defer ts.Close()

	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.iidEndpoint = ts.URL + "/v1"

	resp, err := client.UnsubscribeFromTopicLegacy(ctx, []string{"id1", "id2"}, "test-topic")
	if err != nil {
		t.Fatal(err)
	}
	checkIIDRequest(t, b, tr, iidUnsubscribe)
	checkTopicMgtResponse(t, resp, "error_reason")
}

func TestInvalidSubscribe(t *testing.T) {
	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
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

func TestInvalidUnsubscribe(t *testing.T) {
	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
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

func checkIIDRequest(t *testing.T, b []byte, tr *http.Request, op string) {
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
	wantOp := "/v1:" + op
	if tr.URL.Path != wantOp {
		t.Errorf("Path = %q; want = %q", tr.URL.Path, wantOp)
	}
	if h := tr.Header.Get("Authorization"); h != "Bearer test-token" {
		t.Errorf("Authorization = %q; want = %q", h, "Bearer test-token")
	}
	xGoogAPIClientHeader := internal.GetMetricsHeader(testMessagingConfig.Version)
	if h := tr.Header.Get("x-goog-api-client"); h != xGoogAPIClientHeader {
		t.Errorf("x-goog-api-client header = %q; want = %q", h, xGoogAPIClientHeader)
	}
}

func checkTopicMgtResponse(t *testing.T, resp *TopicManagementResponse, wantReason string) {
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
	if e.Reason != wantReason {
		t.Errorf("ErrorInfo.Reason = %s; want = %s", e.Reason, wantReason)
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
