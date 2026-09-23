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
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"firebase.google.com/go/v4/errorutils"
)

var rfBody = []byte("{\"results\": [{\"apns_token\": \"id1\", \"status\": \"OK\", \"registration_token\": " +
	"\"test-id1\"},{\"apns_token\": \"id1\", \"status\": \"Internal Server Error\"}]}")

func TestGetRegistrationFromAPNs(t *testing.T) {
	var tr *http.Request
	var b []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tr = r
		b, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(rfBody)
	}))
	defer ts.Close()

	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.iidEndpoint = ts.URL + "/v1"

	resp, err := client.GetRegistrationFromAPNs(ctx, "test-app", []string{"id1", "id2"})
	if err != nil {
		t.Fatal(err)
	}
	checkRegistrationFromAPNsRequest(t, b, tr, iidImport, false)
	checkRegistrationFromAPNsResponse(t, resp)
}

func TestGetRegistrationFromAPNsDryRun(t *testing.T) {
	var tr *http.Request
	var b []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tr = r
		b, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(rfBody)
	}))
	defer ts.Close()

	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.iidEndpoint = ts.URL + "/v1"

	resp, err := client.GetRegistrationFromAPNsDryRun(ctx, "test-app", []string{"id1", "id2"})
	if err != nil {
		t.Fatal(err)
	}
	checkRegistrationFromAPNsRequest(t, b, tr, iidImport, true)
	checkRegistrationFromAPNsResponse(t, resp)
}

func TestInvalidGetRegistrationFromAPNs(t *testing.T) {
	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	var invalidArgs = []struct {
		name   string
		tokens []string
		app    string
		want   string
	}{
		{
			name: "NoTokens",
			app:  "app",
			want: "no APNs tokens specified",
		},
		{
			name:   "NoApplicationID",
			tokens: []string{"token1"},
			want:   "application id not specified",
		},
		{
			name:   "TooManyTokens",
			tokens: strings.Split("a"+strings.Repeat(",a", 100), ","),
			app:    "app",
			want:   "too many APNs tokens specified",
		},
		{
			name:   "EmptyToken",
			tokens: []string{"foo", ""},
			app:    "app",
			want:   "tokens list must not contain empty strings",
		},
	}

	for _, tc := range invalidArgs {
		t.Run(tc.name, func(t *testing.T) {
			resp, err2 := client.getRegistrationFromAPNs(ctx, tc.app, tc.tokens, true)
			if err2 == nil || err2.Error() != tc.want {
				t.Errorf(
					"getRegistrationFromAPNs(%s) = (%#v, %v); want = (nil, %q)", tc.name, resp, err2, tc.want)
			}
		})
	}
}

func checkRegistrationFromAPNsRequest(t *testing.T, b []byte, tr *http.Request, op string, sandbox bool) {
	var parsed map[string]interface{}
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatal(err)
	}
	want := map[string]interface{}{
		"application": "test-app",
		"sandbox":     sandbox,
		"apns_tokens": []interface{}{"id1", "id2"},
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
	if h := tr.Header.Get("Content-Type"); h != "application/json" {
		t.Errorf("Content-Type = %q; want = %q", h, "application/json")
	}
	if h := tr.Header.Get("Access_token_auth"); h != "true" {
		t.Errorf("Access_token_auth = %q; want = %q", h, "true")
	}
}

func checkRegistrationFromAPNsResponse(t *testing.T, resp []RegistrationToken) {
	if len(resp) != 2 {
		t.Errorf("RegistrationToken length = %d; want  = %d", len(resp), 2)
	}

	if resp[0].Status != "OK" {
		t.Errorf("Status = %q; want  = %q", resp[0].Status, "OK")
	}
	if resp[1].Status != "Internal Server Error" {
		t.Errorf("Status = %q; want  = %q", resp[1].Status, "Internal Server Error")
	}

	if resp[0].ApnsToken != "id1" {
		t.Errorf("ApnsToken = %q; want  = %q", resp[0].ApnsToken, "id1")
	}
	if resp[1].ApnsToken != "id1" {
		t.Errorf("ApnsToken = %q; want  = %q", resp[1].ApnsToken, "id2")
	}

	if resp[0].RegistrationToken != "test-id1" {
		t.Errorf("ApnsToken = %q; want  = %q", resp[0].RegistrationToken, "test-id1")
	}
	if resp[1].RegistrationToken != "" {
		t.Errorf("ApnsToken = %q; want  = %q", resp[1].RegistrationToken, "")
	}
}

func TestGetTokenDetails(t *testing.T) {
	var tr *http.Request
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tr = r
		if r.Body != http.NoBody {
			t.Errorf("Request body must be empty")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{\"application\":\"com.iid.example\",\"authorizedEntity\":\"123456782354\"," +
			"\"platform\":\"Android\",\"rel\":{\"topics\":{\"topicName1\":{\"addDate\":\"2015-07-30\"}," +
			"\"topicName2\":{\"addDate\":\"2015-07-30\"}}}}"))
	}))
	defer ts.Close()

	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.iidEndpoint = ts.URL + "/v1"

	resp, err := client.GetTokenDetails(ctx, "")
	if err == nil || err.Error() != "token not specified" {
		t.Errorf("GetTokenDetails(EmptyToken) = (%#v, %v); want = (nil, %q)", resp, err, "token not specified")
	}

	resp, err = client.GetTokenDetails(ctx, "id1")
	if err != nil {
		t.Fatal(err)
	}

	if tr.Method != http.MethodGet {
		t.Errorf("Method = %q; want = %q", tr.Method, http.MethodGet)
	}
	if h := tr.Header.Get("Authorization"); h != "Bearer test-token" {
		t.Errorf("Authorization = %q; want = %q", h, "Bearer test-token")
	}
	if h := tr.Header.Get("Access_token_auth"); h != "true" {
		t.Errorf("Access_token_auth = %q; want = %q", h, "true")
	}
	if tr.URL.Path != "/v1:/info/id1" {
		t.Errorf("Path = %q; want = %q", tr.URL.Path, "/v1:/info/id1")
	}

	if resp.Platform != "Android" {
		t.Errorf("Platform = %q; want  = %q", resp.Platform, "Android")
	}
	if resp.Application != "com.iid.example" {
		t.Errorf("Application = %q; want  = %q", resp.Platform, "com.iid.example")
	}
	if resp.AuthorizedEntity != "123456782354" {
		t.Errorf("AuthorizedEntity = %q; want  = %q", resp.Platform, "123456782354")
	}
	if len(resp.Rel.Topics) != 2 {
		t.Errorf("Topics count = %d; want  = %d", len(resp.Rel.Topics), 2)
	}
	if v, exists := resp.Rel.Topics["topicName1"]; exists {
		if v.AddDate != "2015-07-30" {
			t.Errorf("Topics date = %q; want  = %q", v.AddDate, "2015-07-30")
		}
	} else {
		t.Errorf("Topic \"topicname1\" not exists")
	}
}

func TestGetSubscriptions(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != http.NoBody {
			t.Errorf("Request body must be empty")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{\"application\":\"com.iid.example\",\"authorizedEntity\":\"123456782354\"," +
			"\"platform\":\"Android\",\"rel\":{\"topics\":{\"topicName1\":{\"addDate\":\"2015-07-30\"}," +
			"\"topicName2\":{\"addDate\":\"2015-07-30\"}}}}"))
	}))
	defer ts.Close()

	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.iidEndpoint = ts.URL + "/v1"

	resp, err := client.GetSubscriptions(ctx, "id1")
	if err != nil {
		t.Fatal(err)
	}

	if len(resp) != 2 {
		t.Errorf("Topics count = %d; want  = %d", len(resp), 2)
	}
	if v, exists := resp["topicName1"]; exists {
		if v.AddDate != "2015-07-30" {
			t.Errorf("Topics date = %q; want  = %q", v.AddDate, "2015-07-30")
		}
	} else {
		t.Errorf("Topic \"topicname1\" not exists")
	}
}

func TestInvalidResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("{\"error\":\"InvalidToken\"}"))
	}))
	defer ts.Close()

	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.iidEndpoint = ts.URL + "/v1"

	const text = "error while calling the iid service: InvalidToken"

	resp, err := client.GetSubscriptions(ctx, "id1")
	if !errorutils.IsInvalidArgument(err) {
		t.Errorf(
			"GetSubscriptions(InvalidToken) = (%#v, %v); want = (nil, %q)", resp, err, text)
	}
	resp2, err := client.GetRegistrationFromAPNsDryRun(ctx, "test-app", []string{"id1", "id2"})
	if !errorutils.IsInvalidArgument(err) {
		t.Errorf(
			"GetSubscriptions(InvalidToken) = (%#v, %v); want = (nil, %q)", resp2, err, text)
	}
}
