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
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"firebase.google.com/go/v4/errorutils"
)

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
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.iidEndpoint = ts.URL

	resp, err := client.SubscribeToTopic(ctx, []string{"id1", "id2"}, "test-topic")
	if err != nil {
		t.Fatal(err)
	}
	checkIIDRequest(t, b, tr, iidSubscribe)
	checkTopicMgtResponse(t, resp)
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
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.iidEndpoint = ts.URL

	resp, err := client.UnsubscribeFromTopic(ctx, []string{"id1", "id2"}, "test-topic")
	if err != nil {
		t.Fatal(err)
	}
	checkIIDRequest(t, b, tr, iidUnsubscribe)
	checkTopicMgtResponse(t, resp)
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

func TestTopicManagementError(t *testing.T) {
	var resp string
	var status int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(resp))
	}))
	defer ts.Close()

	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.iidEndpoint = ts.URL
	client.iidClient.httpClient.RetryConfig = nil

	cases := []struct {
		name, resp, want string
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
		resp = tc.resp
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
	wantOp := "/" + op
	if tr.URL.Path != wantOp {
		t.Errorf("Path = %q; want = %q", tr.URL.Path, wantOp)
	}
	if h := tr.Header.Get("Authorization"); h != "Bearer test-token" {
		t.Errorf("Authorization = %q; want = %q", h, "Bearer test-token")
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
