// Copyright 2018 Google Inc. All Rights Reserved.
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
	"time"

	"firebase.google.com/go/internal"
	"google.golang.org/api/option"
)

const testMessageID = "projects/test-project/messages/msg_id"

var (
	testMessagingConfig = &internal.MessagingConfig{
		ProjectID: "test-project",
		Opts: []option.ClientOption{
			option.WithTokenSource(&internal.MockTokenSource{AccessToken: "test-token"}),
		},
	}

	ttlWithNanos = time.Duration(1500) * time.Millisecond
	ttl          = time.Duration(10) * time.Second
	invalidTTL   = time.Duration(-10) * time.Second

	badge     = 42
	badgeZero = 0
)

var validMessages = []struct {
	name string
	req  *Message
	want map[string]interface{}
}{
	{
		name: "TokenOnly",
		req:  &Message{Token: "test-token"},
		want: map[string]interface{}{"token": "test-token"},
	},
	{
		name: "TopicOnly",
		req:  &Message{Topic: "test-topic"},
		want: map[string]interface{}{"topic": "test-topic"},
	},
	{
		name: "PrefixedTopicOnly",
		req:  &Message{Topic: "/topics/test-topic"},
		want: map[string]interface{}{"topic": "test-topic"},
	},
	{
		name: "ConditionOnly",
		req:  &Message{Condition: "test-condition"},
		want: map[string]interface{}{"condition": "test-condition"},
	},
	{
		name: "DataMessage",
		req: &Message{
			Data: map[string]string{
				"k1": "v1",
				"k2": "v2",
			},
			Topic: "test-topic",
		},
		want: map[string]interface{}{
			"data": map[string]interface{}{
				"k1": "v1",
				"k2": "v2",
			},
			"topic": "test-topic",
		},
	},
	{
		name: "NotificationMessage",
		req: &Message{
			Notification: &Notification{
				Title: "t",
				Body:  "b",
			},
			Topic: "test-topic",
		},
		want: map[string]interface{}{
			"notification": map[string]interface{}{
				"title": "t",
				"body":  "b",
			},
			"topic": "test-topic",
		},
	},
	{
		name: "AndroidDataMessage",
		req: &Message{
			Android: &AndroidConfig{
				CollapseKey: "ck",
				Data: map[string]string{
					"k1": "v1",
					"k2": "v2",
				},
				Priority: "normal",
				TTL:      &ttl,
			},
			Topic: "test-topic",
		},
		want: map[string]interface{}{
			"android": map[string]interface{}{
				"collapse_key": "ck",
				"data": map[string]interface{}{
					"k1": "v1",
					"k2": "v2",
				},
				"priority": "normal",
				"ttl":      "10s",
			},
			"topic": "test-topic",
		},
	},
	{
		name: "AndroidNotificationMessage",
		req: &Message{
			Android: &AndroidConfig{
				RestrictedPackageName: "rpn",
				Notification: &AndroidNotification{
					Title:        "t",
					Body:         "b",
					Color:        "#112233",
					Sound:        "s",
					TitleLocKey:  "tlk",
					TitleLocArgs: []string{"t1", "t2"},
					BodyLocKey:   "blk",
					BodyLocArgs:  []string{"b1", "b2"},
				},
				TTL: &ttlWithNanos,
			},
			Topic: "test-topic",
		},
		want: map[string]interface{}{
			"android": map[string]interface{}{
				"restricted_package_name": "rpn",
				"notification": map[string]interface{}{
					"title":          "t",
					"body":           "b",
					"color":          "#112233",
					"sound":          "s",
					"title_loc_key":  "tlk",
					"title_loc_args": []interface{}{"t1", "t2"},
					"body_loc_key":   "blk",
					"body_loc_args":  []interface{}{"b1", "b2"},
				},
				"ttl": "1.500000000s",
			},
			"topic": "test-topic",
		},
	},
	{
		name: "AndroidNoTTL",
		req: &Message{
			Android: &AndroidConfig{
				Priority: "high",
			},
			Topic: "test-topic",
		},
		want: map[string]interface{}{
			"android": map[string]interface{}{
				"priority": "high",
			},
			"topic": "test-topic",
		},
	},
	{
		name: "WebpushMessage",
		req: &Message{
			Webpush: &WebpushConfig{
				Headers: map[string]string{
					"h1": "v1",
					"h2": "v2",
				},
				Data: map[string]string{
					"k1": "v1",
					"k2": "v2",
				},
				Notification: &WebpushNotification{
					Title: "t",
					Body:  "b",
					Icon:  "i",
				},
			},
			Topic: "test-topic",
		},
		want: map[string]interface{}{
			"webpush": map[string]interface{}{
				"headers":      map[string]interface{}{"h1": "v1", "h2": "v2"},
				"data":         map[string]interface{}{"k1": "v1", "k2": "v2"},
				"notification": map[string]interface{}{"title": "t", "body": "b", "icon": "i"},
			},
			"topic": "test-topic",
		},
	},
	{
		name: "APNSHeadersOnly",
		req: &Message{
			APNS: &APNSConfig{
				Headers: map[string]string{
					"h1": "v1",
					"h2": "v2",
				},
			},
			Topic: "test-topic",
		},
		want: map[string]interface{}{
			"apns": map[string]interface{}{
				"headers": map[string]interface{}{"h1": "v1", "h2": "v2"},
			},
			"topic": "test-topic",
		},
	},
	{
		name: "APNSAlertString",
		req: &Message{
			APNS: &APNSConfig{
				Headers: map[string]string{
					"h1": "v1",
					"h2": "v2",
				},
				Payload: &APNSPayload{
					Aps: &Aps{
						AlertString:      "a",
						Badge:            &badge,
						Category:         "c",
						Sound:            "s",
						ThreadID:         "t",
						ContentAvailable: true,
					},
					CustomData: map[string]interface{}{
						"k1": "v1",
						"k2": true,
					},
				},
			},
			Topic: "test-topic",
		},
		want: map[string]interface{}{
			"apns": map[string]interface{}{
				"headers": map[string]interface{}{"h1": "v1", "h2": "v2"},
				"payload": map[string]interface{}{
					"aps": map[string]interface{}{
						"alert":             "a",
						"badge":             float64(badge),
						"category":          "c",
						"sound":             "s",
						"thread-id":         "t",
						"content-available": float64(1),
					},
					"k1": "v1",
					"k2": true,
				},
			},
			"topic": "test-topic",
		},
	},
	{
		name: "APNSBadgeZero",
		req: &Message{
			APNS: &APNSConfig{
				Payload: &APNSPayload{
					Aps: &Aps{
						Badge:            &badgeZero,
						Category:         "c",
						Sound:            "s",
						ThreadID:         "t",
						ContentAvailable: true,
					},
				},
			},
			Topic: "test-topic",
		},
		want: map[string]interface{}{
			"apns": map[string]interface{}{
				"payload": map[string]interface{}{
					"aps": map[string]interface{}{
						"badge":             float64(badgeZero),
						"category":          "c",
						"sound":             "s",
						"thread-id":         "t",
						"content-available": float64(1),
					},
				},
			},
			"topic": "test-topic",
		},
	},
	{
		name: "APNSAlertObject",
		req: &Message{
			APNS: &APNSConfig{
				Payload: &APNSPayload{
					Aps: &Aps{
						Alert: &ApsAlert{
							Title:        "t",
							Body:         "b",
							TitleLocKey:  "tlk",
							TitleLocArgs: []string{"t1", "t2"},
							LocKey:       "blk",
							LocArgs:      []string{"b1", "b2"},
							ActionLocKey: "alk",
							LaunchImage:  "li",
						},
					},
				},
			},
			Topic: "test-topic",
		},
		want: map[string]interface{}{
			"apns": map[string]interface{}{
				"payload": map[string]interface{}{
					"aps": map[string]interface{}{
						"alert": map[string]interface{}{
							"title":          "t",
							"body":           "b",
							"title-loc-key":  "tlk",
							"title-loc-args": []interface{}{"t1", "t2"},
							"loc-key":        "blk",
							"loc-args":       []interface{}{"b1", "b2"},
							"action-loc-key": "alk",
							"launch-image":   "li",
						},
					},
				},
			},
			"topic": "test-topic",
		},
	},
}

var invalidMessages = []struct {
	name string
	req  *Message
	want string
}{
	{
		name: "NilMessage",
		req:  nil,
		want: "message must not be nil",
	},
	{
		name: "NoTargets",
		req:  &Message{},
		want: "exactly one of token, topic or condition must be specified",
	},
	{
		name: "MultipleTargets",
		req: &Message{
			Token: "token",
			Topic: "topic",
		},
		want: "exactly one of token, topic or condition must be specified",
	},
	{
		name: "InvalidPrefixedTopicName",
		req: &Message{
			Topic: "/topics/",
		},
		want: "malformed topic name",
	},
	{
		name: "InvalidTopicName",
		req: &Message{
			Topic: "foo*bar",
		},
		want: "malformed topic name",
	},
	{
		name: "InvalidAndroidTTL",
		req: &Message{
			Android: &AndroidConfig{
				TTL: &invalidTTL,
			},
			Topic: "topic",
		},
		want: "ttl duration must not be negative",
	},
	{
		name: "InvalidAndroidPriority",
		req: &Message{
			Android: &AndroidConfig{
				Priority: "not normal",
			},
			Topic: "topic",
		},
		want: "priority must be 'normal' or 'high'",
	},
	{
		name: "InvalidAndroidColor1",
		req: &Message{
			Android: &AndroidConfig{
				Notification: &AndroidNotification{
					Color: "112233",
				},
			},
			Topic: "topic",
		},
		want: "color must be in the #RRGGBB form",
	},
	{
		name: "InvalidAndroidColor2",
		req: &Message{
			Android: &AndroidConfig{
				Notification: &AndroidNotification{
					Color: "#112233X",
				},
			},
			Topic: "topic",
		},
		want: "color must be in the #RRGGBB form",
	},
	{
		name: "InvalidAndroidTitleLocArgs",
		req: &Message{
			Android: &AndroidConfig{
				Notification: &AndroidNotification{
					TitleLocArgs: []string{"a1"},
				},
			},
			Topic: "topic",
		},
		want: "titleLocKey is required when specifying titleLocArgs",
	},
	{
		name: "InvalidAndroidBodyLocArgs",
		req: &Message{
			Android: &AndroidConfig{
				Notification: &AndroidNotification{
					BodyLocArgs: []string{"a1"},
				},
			},
			Topic: "topic",
		},
		want: "bodyLocKey is required when specifying bodyLocArgs",
	},
	{
		name: "APNSMultipleAlerts",
		req: &Message{
			APNS: &APNSConfig{
				Payload: &APNSPayload{
					Aps: &Aps{
						Alert:       &ApsAlert{},
						AlertString: "alert",
					},
				},
			},
			Topic: "topic",
		},
		want: "multiple alert specifications",
	},
	{
		name: "InvalidAPNSTitleLocArgs",
		req: &Message{
			APNS: &APNSConfig{
				Payload: &APNSPayload{
					Aps: &Aps{
						Alert: &ApsAlert{
							TitleLocArgs: []string{"a1"},
						},
					},
				},
			},
			Topic: "topic",
		},
		want: "titleLocKey is required when specifying titleLocArgs",
	},
	{
		name: "InvalidAPNSLocArgs",
		req: &Message{
			APNS: &APNSConfig{
				Payload: &APNSPayload{
					Aps: &Aps{
						Alert: &ApsAlert{
							LocArgs: []string{"a1"},
						},
					},
				},
			},
			Topic: "topic",
		},
		want: "locKey is required when specifying locArgs",
	},
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

func TestNoProjectID(t *testing.T) {
	client, err := NewClient(context.Background(), &internal.MessagingConfig{})
	if client != nil || err == nil {
		t.Errorf("NewClient() = (%v, %v); want = (nil, error)", client, err)
	}
}

func TestSend(t *testing.T) {
	var tr *http.Request
	var b []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tr = r
		b, _ = ioutil.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{ \"name\":\"" + testMessageID + "\" }"))
	}))
	defer ts.Close()

	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.fcmEndpoint = ts.URL

	for _, tc := range validMessages {
		t.Run(tc.name, func(t *testing.T) {
			name, err := client.Send(ctx, tc.req)
			if name != testMessageID || err != nil {
				t.Errorf("Send() = (%q, %v); want = (%q, nil)", name, err, testMessageID)
			}
			checkFCMRequest(t, b, tr, tc.want, false)
		})
	}
}

func TestSendDryRun(t *testing.T) {
	var tr *http.Request
	var b []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tr = r
		b, _ = ioutil.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{ \"name\":\"" + testMessageID + "\" }"))
	}))
	defer ts.Close()

	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.fcmEndpoint = ts.URL

	for _, tc := range validMessages {
		t.Run(tc.name, func(t *testing.T) {
			name, err := client.SendDryRun(ctx, tc.req)
			if name != testMessageID || err != nil {
				t.Errorf("SendDryRun() = (%q, %v); want = (%q, nil)", name, err, testMessageID)
			}
			checkFCMRequest(t, b, tr, tc.want, true)
		})
	}
}

func TestSendError(t *testing.T) {
	var resp string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(resp))
	}))
	defer ts.Close()

	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.fcmEndpoint = ts.URL

	cases := []struct {
		resp string
		want string
	}{
		{
			resp: "{}",
			want: "http error status: 500; reason: server responded with an unknown error; response: {}",
		},
		{
			resp: "{\"error\": {\"status\": \"INVALID_ARGUMENT\", \"message\": \"test error\"}}",
			want: "http error status: 500; reason: request contains an invalid argument; code: invalid-argument",
		},
		{
			resp: "{\"error\": {\"status\": \"NOT_FOUND\", \"message\": \"test error\"}}",
			want: "http error status: 500; reason: app instance has been unregistered; code: registration-token-not-registered",
		},
		{
			resp: "not json",
			want: "http error status: 500; reason: server responded with an unknown error; response: not json",
		},
	}
	for _, tc := range cases {
		resp = tc.resp
		name, err := client.Send(ctx, &Message{Topic: "topic"})
		if err == nil || err.Error() != tc.want {
			t.Errorf("Send() = (%q, %v); want = (%q, %q)", name, err, "", tc.want)
		}
	}
}

func TestInvalidMessage(t *testing.T) {
	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range invalidMessages {
		t.Run(tc.name, func(t *testing.T) {
			name, err := client.Send(ctx, tc.req)
			if err == nil || err.Error() != tc.want {
				t.Errorf("Send() = (%q, %v); want = (%q, %q)", name, err, "", tc.want)
			}
		})
	}
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
			name, err := client.SubscribeToTopic(ctx, tc.tokens, tc.topic)
			if err == nil || err.Error() != tc.want {
				t.Errorf("SubscribeToTopic() = (%q, %v); want = (%q, %q)", name, err, "", tc.want)
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
			name, err := client.UnsubscribeFromTopic(ctx, tc.tokens, tc.topic)
			if err == nil || err.Error() != tc.want {
				t.Errorf("UnsubscribeFromTopic() = (%q, %v); want = (%q, %q)", name, err, "", tc.want)
			}
		})
	}
}

func TestTopicManagementError(t *testing.T) {
	var resp string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
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

	cases := []struct {
		resp string
		want string
	}{
		{
			resp: "{}",
			want: "http error status: 500; reason: client encountered an unknown error; response: {}",
		},
		{
			resp: "{\"error\": \"INVALID_ARGUMENT\"}",
			want: "http error status: 500; reason: request contains an invalid argument; code: invalid-argument",
		},
		{
			resp: "not json",
			want: "http error status: 500; reason: client encountered an unknown error; response: not json",
		},
	}
	for _, tc := range cases {
		resp = tc.resp
		tmr, err := client.SubscribeToTopic(ctx, []string{"id1"}, "topic")
		if err == nil || err.Error() != tc.want {
			t.Errorf("SubscribeToTopic() = (%q, %v); want = (%q, %q)", tmr, err, "", tc.want)
		}
	}
	for _, tc := range cases {
		resp = tc.resp
		tmr, err := client.UnsubscribeFromTopic(ctx, []string{"id1"}, "topic")
		if err == nil || err.Error() != tc.want {
			t.Errorf("UnsubscribeFromTopic() = (%q, %v); want = (%q, %q)", tmr, err, "", tc.want)
		}
	}
}

func checkFCMRequest(t *testing.T, b []byte, tr *http.Request, want map[string]interface{}, dryRun bool) {
	var parsed map[string]interface{}
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(parsed["message"], want) {
		t.Errorf("Body = %#v; want = %#v", parsed["message"], want)
	}

	validate, ok := parsed["validate_only"]
	if dryRun {
		if !ok || validate != true {
			t.Errorf("ValidateOnly = %v; want = true", validate)
		}
	} else if ok {
		t.Errorf("ValidateOnly = %v; want none", validate)
	}

	if tr.Method != http.MethodPost {
		t.Errorf("Method = %q; want = %q", tr.Method, http.MethodPost)
	}
	if tr.URL.Path != "/projects/test-project/messages:send" {
		t.Errorf("Path = %q; want = %q", tr.URL.Path, "/projects/test-project/messages:send")
	}
	if h := tr.Header.Get("Authorization"); h != "Bearer test-token" {
		t.Errorf("Authorization = %q; want = %q", h, "Bearer test-token")
	}
}

func checkIIDRequest(t *testing.T, b []byte, tr *http.Request, op string) {
	var parsed map[string]interface{}
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatal(err)
	}
	want := map[string]interface{}{
		"to": "/topics/test-topic",
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
	if e.Reason != "unknown-error" {
		t.Errorf("ErrorInfo.Reason = %s; want = %s", e.Reason, "unknown-error")
	}
}
