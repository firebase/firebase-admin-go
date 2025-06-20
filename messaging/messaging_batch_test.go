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
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	// "errors" // Unused
	"fmt"
	"io" // Add back io import
	"io/ioutil"
	"log"
	// "mime" // Unused by top-level test functions
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	// "sync" // Unused by top-level test functions
	"testing"

	"firebase.google.com/go/v4/app" // Import app package
	"firebase.google.com/go/v4/internal"
	"google.golang.org/api/option"
)

// testProjectID is used for app initialization in tests.
// It's also used in constructing wantSendURL.
const testMessagingProjectID = "test-project"

var testMessages = []*Message{
	{Topic: "topic1"},
	{Topic: "topic2"},
}
var testMulticastMessage = &MulticastMessage{
	Tokens: []string{"token1", "token2"},
}
var testSuccessResponse = []fcmResponse{
	{
		Name: fmt.Sprintf("projects/%s/messages/1", testMessagingProjectID),
	},
	{
		Name: fmt.Sprintf("projects/%s/messages/2", testMessagingProjectID),
	},
}

const wantMime = "multipart/mixed; boundary=__END_OF_PART__"
const wantSendURL = "/v1/projects/" + testMessagingProjectID + "/messages:send" // Updated to use const

// Helper to create a new app.App for Messaging tests
func newTestMessagingAppForBatch(ctx context.Context) *app.App {
	opts := []option.ClientOption{
		option.WithTokenSource(&internal.MockTokenSource{AccessToken: "test-token"}),
		option.WithScopes(internal.FirebaseScopes...),
	}
	appInstance, err := app.New(ctx, &app.Config{ProjectID: testMessagingProjectID}, opts...)
	if err != nil {
		log.Fatalf("Error creating test app for Messaging (batch): %v", err)
	}
	return appInstance
}

func TestMultipartEntitySingle(t *testing.T) {
	entity := &multipartEntity{
		parts: []*part{
			{
				method: "POST",
				url:    "http://example.com",
				body:   map[string]interface{}{"key": "value"},
			},
		},
	}

	mime := entity.Mime()
	if mime != wantMime {
		t.Errorf("Mime() = %q; want = %q", mime, wantMime)
	}

	b, err := entity.Bytes()
	if err != nil {
		t.Fatal(err)
	}

	want := "--__END_OF_PART__\r\n" +
		"Content-Id: 1\r\n" +
		"Content-Length: 120\r\n" +
		"Content-Transfer-Encoding: binary\r\n" +
		"Content-Type: application/http\r\n" +
		"\r\n" +
		"POST / HTTP/1.1\r\n" +
		"Host: example.com\r\n" +
		"Content-Length: 15\r\n" +
		"Content-Type: application/json; charset=UTF-8\r\n" +
		"\r\n" +
		"{\"key\":\"value\"}\r\n" +
		"--__END_OF_PART__--\r\n"
	if string(b) != want {
		t.Errorf("Bytes() = %q; want = %q", string(b), want)
	}
}

func TestMultipartEntity(t *testing.T) {
	entity := &multipartEntity{
		parts: []*part{
			{
				method: "POST",
				url:    "http://example1.com",
				body:   map[string]interface{}{"key1": "value"},
			},
			{
				method:  "POST",
				url:     "http://example2.com",
				body:    map[string]interface{}{"key2": "value"},
				headers: map[string]string{"Custom-Header": "custom-value"},
			},
		},
	}

	mime := entity.Mime()
	if mime != wantMime {
		t.Errorf("Mime() = %q; want = %q", mime, wantMime)
	}

	b, err := entity.Bytes()
	if err != nil {
		t.Fatal(err)
	}

	want := "--__END_OF_PART__\r\n" +
		"Content-Id: 1\r\n" +
		"Content-Length: 122\r\n" +
		"Content-Transfer-Encoding: binary\r\n" +
		"Content-Type: application/http\r\n" +
		"\r\n" +
		"POST / HTTP/1.1\r\n" +
		"Host: example1.com\r\n" +
		"Content-Length: 16\r\n" +
		"Content-Type: application/json; charset=UTF-8\r\n" +
		"\r\n" +
		"{\"key1\":\"value\"}\r\n" +
		"--__END_OF_PART__\r\n" +
		"Content-Id: 2\r\n" +
		"Content-Length: 151\r\n" +
		"Content-Transfer-Encoding: binary\r\n" +
		"Content-Type: application/http\r\n" +
		"\r\n" +
		"POST / HTTP/1.1\r\n" +
		"Host: example2.com\r\n" +
		"Content-Length: 16\r\n" +
		"Content-Type: application/json; charset=UTF-8\r\n" +
		"Custom-Header: custom-value\r\n" +
		"\r\n" +
		"{\"key2\":\"value\"}\r\n" +
		"--__END_OF_PART__--\r\n"
	if string(b) != want {
		t.Errorf("multipartPayload() = %q; want = %q", string(b), want)
	}
}

func TestMultipartEntityError(t *testing.T) {
	entity := &multipartEntity{
		parts: []*part{
			{
				method: "POST",
				url:    "http://example.com",
				body:   func() {},
			},
		},
	}

	b, err := entity.Bytes()
	if b != nil || err == nil {
		t.Errorf("Bytes() = (%v, %v); want = (nil, error)", b, nil)
	}
}

func TestSendEachEmptyArray(t *testing.T) {
	ctx := context.Background()
	appInstance := newTestMessagingAppForBatch(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}

	want := "messages must not be nil or empty"
	br, err := client.SendEach(ctx, nil)
	if err == nil || err.Error() != want {
		t.Errorf("SendEach(nil) = (%v, %v); want = (nil, %q)", br, err, want)
	}

	br, err = client.SendEach(ctx, []*Message{})
	if err == nil || err.Error() != want {
		t.Errorf("SendEach(nil) = (%v, %v); want = (nil, %q)", br, err, want)
	}
}

func TestSendEachTooManyMessages(t *testing.T) {
	ctx := context.Background()
	appInstance := newTestMessagingAppForBatch(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}

	var messages []*Message
	for i := 0; i < 501; i++ {
		messages = append(messages, &Message{Topic: "test-topic"})
	}

	want := "messages must not contain more than 500 elements"
	br, err := client.SendEach(ctx, messages)
	if err == nil || err.Error() != want {
		t.Errorf("SendEach() = (%v, %v); want = (nil, %q)", br, err, want)
	}
}

func TestSendEachInvalidMessage(t *testing.T) {
	ctx := context.Background()
	appInstance := newTestMessagingAppForBatch(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}

	want := "invalid message at index 0: message must not be nil"
	br, err := client.SendEach(ctx, []*Message{nil})
	if err == nil || err.Error() != want {
		t.Errorf("SendEach() = (%v, %v); want = (nil, %q)", br, err, want)
	}
}

func TestSendEach(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, _ := ioutil.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		for idx, testMessage := range testMessages {
			if strings.Contains(string(req), testMessage.Topic) {
				// Ensure response name includes the correct project ID
				w.Write([]byte(fmt.Sprintf(`{ "name":"projects/%s/messages/%d" }`, testMessagingProjectID, idx+1)))
				return // Return after finding the matching topic to avoid writing multiple responses
			}
		}
		// Fallback if no topic matches (should not happen in this test's logic)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"no matching topic found in request"}`))
	}))
	defer ts.Close()
	ctx := context.Background()
	appInstance := newTestMessagingAppForBatch(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}
	client.fcmClient.fcmEndpoint = ts.URL

	br, err := client.SendEach(ctx, testMessages)
	if err != nil {
		t.Fatal(err)
	}

	if err := checkSuccessfulBatchResponseForSendEach(br, false); err != nil {
		t.Errorf("SendEach() = %v", err)
	}
}

func TestSendEachDryRun(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, _ := ioutil.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		for idx, testMessage := range testMessages {
			if strings.Contains(string(req), testMessage.Topic) {
				w.Write([]byte(fmt.Sprintf(`{ "name":"projects/%s/messages/%d" }`, testMessagingProjectID, idx+1)))
				return
			}
		}
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"no matching topic found in request for dry run"}`))
	}))
	defer ts.Close()
	ctx := context.Background()
	appInstance := newTestMessagingAppForBatch(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}
	client.fcmClient.fcmEndpoint = ts.URL

	br, err := client.SendEachDryRun(ctx, testMessages)
	if err != nil {
		t.Fatal(err)
	}

	if err := checkSuccessfulBatchResponseForSendEach(br, true); err != nil {
		t.Errorf("SendEachDryRun() = %v", err) // Corrected test name in error
	}
}


func TestSendEachPartialFailure(t *testing.T) {
	var failures []string // To store the error response for the failing part
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqBodyBytes, _ := ioutil.ReadAll(r.Body)
		reqBodyStr := string(reqBodyBytes)

		// Determine which message this request is for (based on topic in this test's case)
		// This logic assumes SendEach sends individual requests that can be distinguished.
		if strings.Contains(reqBodyStr, testMessages[0].Topic) { // topic1
			w.Header().Set("Content-Type", "application/json")
			// Use testSuccessResponse[0].Name which already includes projectID
			w.Write([]byte(fmt.Sprintf(`{ "name":"%s" }`, testSuccessResponse[0].Name)))
		} else if strings.Contains(reqBodyStr, testMessages[1].Topic) { // topic2 - this one will fail
			w.WriteHeader(http.StatusInternalServerError) // Or any other error status
			w.Header().Set("Content-Type", "application/json")
			if len(failures) > 0 {
				w.Write([]byte(failures[0]))
			} else {
				w.Write([]byte(`{"error":{"message":"simulated error"}}`)) // Default error
			}
		} else {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"unknown topic in request"}`))
		}
	}))
	defer ts.Close()

	ctx := context.Background()
	appInstance := newTestMessagingAppForBatch(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}
	client.fcmClient.fcmEndpoint = ts.URL // Point to mock server

	for idx, tc := range httpErrors {
		failures = []string{tc.resp} // Set the error response for the failing call

		br, err := client.SendEach(ctx, testMessages) // testMessages has 2 messages
		if err != nil {
			// SendEach itself shouldn't error on partial failures, results are in BatchResponse
			t.Fatalf("[%d] SendEach() returned error: %v", idx, err)
		}

		// Expect 1 success (for topic1) and 1 failure (for topic2 with tc.resp)
		if br.SuccessCount != 1 || br.FailureCount != 1 || len(br.Responses) != 2 {
			t.Errorf("[%d] SendEach() response counts incorrect: Success=%d (want 1), Failure=%d (want 1), Total=%d (want 2)",
				idx, br.SuccessCount, br.FailureCount, len(br.Responses))
			continue
		}

		// Check first response (success)
		if err := checkSuccessfulSendResponse(br.Responses[0], testSuccessResponse[0].Name); err != nil {
			t.Errorf("[%d] SendEach() success response check failed: %v", idx, err)
		}

		// Check second response (failure)
		failureResp := br.Responses[1]
		if failureResp.Success {
			t.Errorf("[%d] SendEach() failure response marked as success", idx)
		}
		if failureResp.Error == nil || failureResp.Error.Error() != tc.want || !tc.check(failureResp.Error) {
			t.Errorf("[%d] SendEach() failure response error mismatch: got %v, want %q (with check %T)", idx, failureResp.Error, tc.want, tc.check)
		}
	}
}


func TestSendEachTotalFailure(t *testing.T) {
	var respBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(respBody))
	}))
	defer ts.Close()

	ctx := context.Background()
	appInstance := newTestMessagingAppForBatch(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}
	client.fcmClient.fcmEndpoint = ts.URL
	if client.fcmClient.httpClient != nil { // Nullify retry for tests expecting immediate failure
	client.fcmClient.httpClient.RetryConfig = nil
	}


	for idx, tc := range httpErrors {
		respBody = tc.resp // Set the error response for all calls
		br, err := client.SendEach(ctx, testMessages) // testMessages has 2 messages
		if err != nil {
			// SendEach itself shouldn't error unless it's a catastrophic failure before any attempt.
			// Individual errors are in BatchResponse.
			t.Fatalf("[%d] SendEach() returned error: %v", idx, err)
		}

		// Expect 0 successes and 2 failures
		if br.SuccessCount != 0 || br.FailureCount != 2 || len(br.Responses) != 2 {
			t.Errorf("[%d] SendEach() response counts incorrect: Success=%d (want 0), Failure=%d (want 2), Total=%d (want 2)",
				idx, br.SuccessCount, br.FailureCount, len(br.Responses))
			continue
		}

		for i, r := range br.Responses {
			if r.Success {
				t.Errorf("[%d] SendEach() response %d marked as success; want failure", idx, i)
			}
			if r.Error == nil || r.Error.Error() != tc.want || !tc.check(r.Error) {
				t.Errorf("[%d] SendEach() response %d error mismatch: got %v, want %q", idx, i, r.Error, tc.want)
			}
		}
	}
}

func TestSendEachForMulticastNil(t *testing.T) {
	ctx := context.Background()
	appInstance := newTestMessagingAppForBatch(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}

	want := "message must not be nil"
	br, err := client.SendEachForMulticast(ctx, nil)
	if err == nil || err.Error() != want {
		t.Errorf("SendEachForMulticast(nil) = (%v, %v); want = (nil, %q)", br, err, want)
	}

	br, err = client.SendEachForMulticastDryRun(ctx, nil)
	if err == nil || err.Error() != want {
		t.Errorf("SendEachForMulticastDryRun(nil) = (%v, %v); want = (nil, %q)", br, err, want) // Corrected test name
	}
}

func TestSendEachForMulticastEmptyArray(t *testing.T) {
	ctx := context.Background()
	appInstance := newTestMessagingAppForBatch(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}

	want := "tokens must not be nil or empty"
	mm := &MulticastMessage{}
	br, err := client.SendEachForMulticast(ctx, mm)
	if err == nil || err.Error() != want {
		t.Errorf("SendEachForMulticast(Tokens: nil) = (%v, %v); want = (nil, %q)", br, err, want)
	}

	var tokens []string
	mm = &MulticastMessage{
		Tokens: tokens,
	}
	br, err = client.SendEachForMulticast(ctx, mm)
	if err == nil || err.Error() != want {
		t.Errorf("SendEachForMulticast(Tokens: []) = (%v, %v); want = (nil, %q)", br, err, want)
	}
}

func TestSendEachForMulticastTooManyTokens(t *testing.T) {
	ctx := context.Background()
	appInstance := newTestMessagingAppForBatch(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}

	var tokens []string
	for i := 0; i < 501; i++ {
		tokens = append(tokens, fmt.Sprintf("token%d", i))
	}

	want := "tokens must not contain more than 500 elements"
	mm := &MulticastMessage{Tokens: tokens}
	br, err := client.SendEachForMulticast(ctx, mm)
	if err == nil || err.Error() != want {
		t.Errorf("SendEachForMulticast() = (%v, %v); want = (nil, %q)", br, err, want)
	}
}

func TestSendEachForMulticastInvalidMessage(t *testing.T) {
	ctx := context.Background()
	appInstance := newTestMessagingAppForBatch(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}

	want := "invalid message at index 0: priority must be 'normal' or 'high'"
	mm := &MulticastMessage{
		Tokens: []string{"token1"},
		Android: &AndroidConfig{
			Priority: "invalid",
		},
	}
	br, err := client.SendEachForMulticast(ctx, mm)
	if err == nil || err.Error() != want {
		t.Errorf("SendEachForMulticast() = (%v, %v); want = (nil, %q)", br, err, want)
	}
}

func TestSendEachForMulticast(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, _ := ioutil.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		for idx, token := range testMulticastMessage.Tokens {
			if strings.Contains(string(req), token) {
				w.Write([]byte(fmt.Sprintf(`{ "name":"projects/%s/messages/%d" }`, testMessagingProjectID, idx+1)))
				return
			}
		}
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"no matching token in request"}`))
	}))
	defer ts.Close()
	ctx := context.Background()
	appInstance := newTestMessagingAppForBatch(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}
	client.fcmClient.fcmEndpoint = ts.URL

	br, err := client.SendEachForMulticast(ctx, testMulticastMessage)
	if err != nil {
		t.Fatal(err)
	}

	if err := checkSuccessfulBatchResponseForSendEach(br, false); err != nil {
		t.Errorf("SendEachForMulticast() = %v", err)
	}
}

func TestSendEachForMulticastWithCustomEndpoint(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, _ := ioutil.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		for idx, token := range testMulticastMessage.Tokens {
			if strings.Contains(string(req), token) {
				w.Write([]byte(fmt.Sprintf(`{ "name":"projects/%s/messages/%d" }`, testMessagingProjectID, idx+1)))
				return
			}
		}
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"no matching token in request"}`))
	}))
	defer ts.Close()

	ctx := context.Background()
	appOpts := []option.ClientOption{
		option.WithTokenSource(&internal.MockTokenSource{AccessToken: "test-token"}),
		option.WithEndpoint(ts.URL),
		option.WithScopes(internal.FirebaseScopes...),
	}
	appInstance, err := app.New(ctx, &app.Config{ProjectID: testMessagingProjectID}, appOpts...)
	if err != nil {
		t.Fatal(err)
	}

	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}

	if ts.URL != client.fcmClient.fcmEndpoint {
		t.Errorf("client.fcmClient.fcmEndpoint = %q; want = %q", client.fcmClient.fcmEndpoint, ts.URL)
	}
	// Note: batchEndpoint might also be set by WithEndpoint if not handled separately by NewClient
	if ts.URL != client.fcmClient.batchEndpoint {
		t.Errorf("client.fcmClient.batchEndpoint = %q; want = %q", client.fcmClient.batchEndpoint, ts.URL)
	}


	br, err := client.SendEachForMulticast(ctx, testMulticastMessage)
	if err := checkSuccessfulBatchResponseForSendEach(br, false); err != nil { // checking err from SendEachForMulticast
		t.Errorf("SendEachForMulticast() = %v", err)
	}
}

func TestSendEachForMulticastDryRun(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, _ := ioutil.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		for idx, token := range testMulticastMessage.Tokens {
			if strings.Contains(string(req), token) {
				w.Write([]byte(fmt.Sprintf(`{ "name":"projects/%s/messages/%d" }`, testMessagingProjectID, idx+1)))
				return
			}
		}
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"no matching token in request"}`))
	}))
	defer ts.Close()
	ctx := context.Background()
	appInstance := newTestMessagingAppForBatch(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}
	client.fcmClient.fcmEndpoint = ts.URL

	br, err := client.SendEachForMulticastDryRun(ctx, testMulticastMessage)
	if err != nil {
		t.Fatal(err)
	}

	if err := checkSuccessfulBatchResponseForSendEach(br, true); err != nil {
		t.Errorf("SendEachForMulticastDryRun() = %v", err)
	}
}

func TestSendEachForMulticastPartialFailure(t *testing.T) {
	var failures []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqBodyBytes, _ := ioutil.ReadAll(r.Body)
		reqBodyStr := string(reqBodyBytes)

		if strings.Contains(reqBodyStr, testMulticastMessage.Tokens[0]) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(fmt.Sprintf(`{ "name":"%s" }`, testSuccessResponse[0].Name)))
		} else if strings.Contains(reqBodyStr, testMulticastMessage.Tokens[1]) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Header().Set("Content-Type", "application/json")
			if len(failures) > 0 {
				w.Write([]byte(failures[0]))
			} else {
				w.Write([]byte(`{"error":{"message":"simulated error for token2"}}`))
			}
		} else {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"unknown token in request"}`))
		}
	}))
	defer ts.Close()

	ctx := context.Background()
	appInstance := newTestMessagingAppForBatch(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}
	client.fcmClient.fcmEndpoint = ts.URL

	for idx, tc := range httpErrors {
		failures = []string{tc.resp}

		br, err := client.SendEachForMulticast(ctx, testMulticastMessage)
		if err != nil {
			t.Fatalf("[%d] SendEachForMulticast() returned error: %v", idx, err)
		}

		if err := checkPartialErrorBatchResponse(br, tc); err != nil { // This check needs to be specific to this test
			t.Errorf("[%d] SendEachForMulticast() batch response check failed: %v", idx, err)
		}
	}
}


func TestSendAllEmptyArray(t *testing.T) {
	ctx := context.Background()
	appInstance := newTestMessagingAppForBatch(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}

	want := "messages must not be nil or empty"
	br, err := client.SendAll(ctx, nil)
	if err == nil || err.Error() != want {
		t.Errorf("SendAll(nil) = (%v, %v); want = (nil, %q)", br, err, want)
	}

	br, err = client.SendAll(ctx, []*Message{})
	if err == nil || err.Error() != want {
		t.Errorf("SendAll([]) = (%v, %v); want = (nil, %q)", br, err, want) // Corrected from SendAll(nil)
	}
}

func TestSendAllTooManyMessages(t *testing.T) {
	ctx := context.Background()
	appInstance := newTestMessagingAppForBatch(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}

	var messages []*Message
	for i := 0; i < 501; i++ {
		messages = append(messages, &Message{Topic: "test-topic"})
	}

	want := "messages must not contain more than 500 elements"
	br, err := client.SendAll(ctx, messages)
	if err == nil || err.Error() != want {
		t.Errorf("SendAll() = (%v, %v); want = (nil, %q)", br, err, want)
	}
}

func TestSendAllInvalidMessage(t *testing.T) {
	ctx := context.Background()
	appInstance := newTestMessagingAppForBatch(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}

	want := "invalid message at index 0: message must not be nil"
	br, err := client.SendAll(ctx, []*Message{nil})
	if err == nil || err.Error() != want {
		t.Errorf("SendAll() = (%v, %v); want = (nil, %q)", br, err, want)
	}
}

func TestSendAll(t *testing.T) {
	resp, err := createMultipartResponse(testSuccessResponse, nil)
	if err != nil {
		t.Fatal(err)
	}

	var req []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, _ = ioutil.ReadAll(r.Body)
		w.Header().Set("Content-Type", wantMime)
		w.Write(resp)
	}))
	defer ts.Close()

	ctx := context.Background()
	appInstance := newTestMessagingAppForBatch(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}
	client.fcmClient.batchEndpoint = ts.URL

	br, err := client.SendAll(ctx, testMessages)
	if err != nil {
		t.Fatal(err)
	}

	if err := checkSuccessfulBatchResponse(br, req, false, appInstance.SDKVersion()); err != nil {
		t.Errorf("SendAll() = %v", err)
	}
}

func TestSendAllDryRun(t *testing.T) {
	resp, err := createMultipartResponse(testSuccessResponse, nil)
	if err != nil {
		t.Fatal(err)
	}

	var req []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, _ = ioutil.ReadAll(r.Body)
		w.Header().Set("Content-Type", wantMime)
		w.Write(resp)
	}))
	defer ts.Close()

	ctx := context.Background()
	appInstance := newTestMessagingAppForBatch(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}
	client.fcmClient.batchEndpoint = ts.URL

	br, err := client.SendAllDryRun(ctx, testMessages)
	if err != nil {
		t.Fatal(err)
	}

	if err := checkSuccessfulBatchResponse(br, req, true, appInstance.SDKVersion()); err != nil {
		t.Errorf("SendAllDryRun() = %v", err) // Corrected test name
	}
}

func TestSendAllPartialFailure(t *testing.T) {
	success := []fcmResponse{
		testSuccessResponse[0], // Use the first predefined success response
	}

	var reqBody, respBody []byte // reqBody for request, respBody for response
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqBody, _ = ioutil.ReadAll(r.Body)
		w.Header().Set("Content-Type", wantMime)
		w.Write(respBody) // Write the pre-constructed multipart response
	}))
	defer ts.Close()

	ctx := context.Background()
	appInstance := newTestMessagingAppForBatch(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}
	client.fcmClient.batchEndpoint = ts.URL

	for idx, tc := range httpErrors {
		failures := []string{tc.resp}
		respBody, err = createMultipartResponse(success, failures) // Construct multipart response
		if err != nil {
			t.Fatalf("[%d] Failed to create multipart response: %v", idx, err)
		}

		br, err := client.SendAll(ctx, testMessages) // testMessages has 2 messages
		if err != nil {
			// SendAll itself shouldn't error on partial failures
			t.Fatalf("[%d] SendAll() returned error: %v", idx, err)
		}

		if err := checkPartialErrorBatchResponse(br, tc); err != nil {
			t.Errorf("[%d] SendAll() batch response check failed: %v", idx, err)
		}

		if err := checkMultipartRequest(reqBody, false, appInstance.SDKVersion()); err != nil {
			t.Errorf("[%d] MultipartRequest check failed: %v", idx, err)
		}
	}
}


func TestSendAllTotalFailure(t *testing.T) {
	var respBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(respBody))
	}))
	defer ts.Close()

	ctx := context.Background()
	appInstance := newTestMessagingAppForBatch(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}
	client.fcmClient.batchEndpoint = ts.URL
	if client.fcmClient.httpClient != nil {
		client.fcmClient.httpClient.RetryConfig = nil
	}


	for _, tc := range httpErrors {
		respBody = tc.resp
		br, err := client.SendAll(ctx, []*Message{{Topic: "topic"}}) // Send a single message for this test
		if err == nil || err.Error() != tc.want || !tc.check(err) {
			t.Errorf("SendAll() = (%v, %v); want = (nil, %q)", br, err, tc.want)
		}
	}
}

func TestSendAllNonMultipartResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{}"))
	}))
	defer ts.Close()

	ctx := context.Background()
	appInstance := newTestMessagingAppForBatch(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}
	client.fcmClient.batchEndpoint = ts.URL
	if _, err = client.SendAll(ctx, testMessages); err == nil {
		t.Fatal("SendAll() = nil; want = error")
	}
}

func TestSendAllMalformedContentType(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "invalid content-type")
		w.Write([]byte("{}"))
	}))
	defer ts.Close()

	ctx := context.Background()
	appInstance := newTestMessagingAppForBatch(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}
	client.fcmClient.batchEndpoint = ts.URL
	if _, err = client.SendAll(ctx, testMessages); err == nil {
		t.Fatal("SendAll() = nil; want = error")
	}
}

func TestSendAllMalformedMultipartResponse(t *testing.T) {
	malformedResp := "--__END_OF_PART__\r\n" +
		"Content-Id: 1\r\n" +
		"Content-Type: application/http\r\n" +
		"\r\n" +
		"Malformed Response\r\n" +
		"--__END_OF_PART__--\r\n"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", wantMime)
		w.Write([]byte(malformedResp))
	}))
	defer ts.Close()

	ctx := context.Background()
	appInstance := newTestMessagingAppForBatch(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}
	client.fcmClient.batchEndpoint = ts.URL
	if _, err = client.SendAll(ctx, testMessages); err == nil {
		t.Fatal("SendAll() = nil; want = error")
	}
}

func TestSendMulticastNil(t *testing.T) {
	ctx := context.Background()
	appInstance := newTestMessagingAppForBatch(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}

	want := "message must not be nil"
	br, err := client.SendMulticast(ctx, nil)
	if err == nil || err.Error() != want {
		t.Errorf("SendMulticast(nil) = (%v, %v); want = (nil, %q)", br, err, want)
	}

	br, err = client.SendMulticastDryRun(ctx, nil)
	if err == nil || err.Error() != want {
		t.Errorf("SendMulticastDryRun(nil) = (%v, %v); want = (nil, %q)", br, err, want) // Corrected test name
	}
}

func TestSendMulticastEmptyArray(t *testing.T) {
	ctx := context.Background()
	appInstance := newTestMessagingAppForBatch(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}

	want := "tokens must not be nil or empty"
	mm := &MulticastMessage{}
	br, err := client.SendMulticast(ctx, mm)
	if err == nil || err.Error() != want {
		t.Errorf("SendMulticast(Tokens: nil) = (%v, %v); want = (nil, %q)", br, err, want)
	}

	var tokens []string
	mm = &MulticastMessage{
		Tokens: tokens,
	}
	br, err = client.SendMulticast(ctx, mm)
	if err == nil || err.Error() != want {
		t.Errorf("SendMulticast(Tokens: []) = (%v, %v); want = (nil, %q)", br, err, want)
	}
}

func TestSendMulticastTooManyTokens(t *testing.T) {
	ctx := context.Background()
	appInstance := newTestMessagingAppForBatch(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}

	var tokens []string
	for i := 0; i < 501; i++ {
		tokens = append(tokens, fmt.Sprintf("token%d", i))
	}

	want := "tokens must not contain more than 500 elements"
	mm := &MulticastMessage{Tokens: tokens}
	br, err := client.SendMulticast(ctx, mm)
	if err == nil || err.Error() != want {
		t.Errorf("SendMulticast() = (%v, %v); want = (nil, %q)", br, err, want)
	}
}

func TestSendMulticastInvalidMessage(t *testing.T) {
	ctx := context.Background()
	appInstance := newTestMessagingAppForBatch(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}

	want := "invalid message at index 0: priority must be 'normal' or 'high'"
	mm := &MulticastMessage{
		Tokens: []string{"token1"},
		Android: &AndroidConfig{
			Priority: "invalid",
		},
	}
	br, err := client.SendMulticast(ctx, mm)
	if err == nil || err.Error() != want {
		t.Errorf("SendMulticast() = (%v, %v); want = (nil, %q)", br, err, want)
	}
}

func TestSendMulticast(t *testing.T) {
	resp, err := createMultipartResponse(testSuccessResponse, nil)
	if err != nil {
		t.Fatal(err)
	}

	var req []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, _ = ioutil.ReadAll(r.Body)
		w.Header().Set("Content-Type", wantMime)
		w.Write(resp)
	}))
	defer ts.Close()

	ctx := context.Background()
	appInstance := newTestMessagingAppForBatch(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}
	client.fcmClient.batchEndpoint = ts.URL

	br, err := client.SendMulticast(ctx, testMulticastMessage)
	if err != nil {
		t.Fatal(err)
	}

	if err := checkSuccessfulBatchResponse(br, req, false, appInstance.SDKVersion()); err != nil {
		t.Errorf("SendMulticast() = %v", err)
	}
}

func TestSendMulticastWithCustomEndpoint(t *testing.T) {
	resp, err := createMultipartResponse(testSuccessResponse, nil)
	if err != nil {
		t.Fatal(err)
	}

	var req []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, _ = ioutil.ReadAll(r.Body)
		w.Header().Set("Content-Type", wantMime)
		w.Write(resp)
	}))
	defer ts.Close()

	ctx := context.Background()

	// Create app with custom endpoint for batch
	// Note: messaging NewClient logic uses app.Options() to get endpoint.
	// If a single endpoint is provided, it's used for both fcmEndpoint and batchEndpoint.
	appOpts := []option.ClientOption{
		option.WithTokenSource(&internal.MockTokenSource{AccessToken: "test-token"}),
		option.WithEndpoint(ts.URL), // This sets the endpoint for the HTTP client in appInstance
		option.WithScopes(internal.FirebaseScopes...),
	}
	appInstance, err := app.New(ctx, &app.Config{ProjectID: testMessagingProjectID}, appOpts...)
	if err != nil {
		t.Fatal(err)
	}

	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}

	// Verify the batchEndpoint was set from the app's options
	if ts.URL != client.fcmClient.batchEndpoint {
		t.Errorf("client.fcmClient.batchEndpoint = %q; want = %q", client.fcmClient.batchEndpoint, ts.URL)
	}

	br, err := client.SendMulticast(ctx, testMulticastMessage)
	if err != nil { // Check err from SendMulticast
		t.Fatal(err)
	}

	if err := checkSuccessfulBatchResponse(br, req, false, appInstance.SDKVersion()); err != nil {
		t.Errorf("SendMulticast() batch response check failed: %v", err)
	}
}


func TestSendMulticastDryRun(t *testing.T) {
	resp, err := createMultipartResponse(testSuccessResponse, nil)
	if err != nil {
		t.Fatal(err)
	}

	var req []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, _ = ioutil.ReadAll(r.Body)
		w.Header().Set("Content-Type", wantMime)
		w.Write(resp)
	}))
	defer ts.Close()

	ctx := context.Background()
	appInstance := newTestMessagingAppForBatch(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}
	client.fcmClient.batchEndpoint = ts.URL

	br, err := client.SendMulticastDryRun(ctx, testMulticastMessage)
	if err != nil {
		t.Fatal(err)
	}

	if err := checkSuccessfulBatchResponse(br, req, true, appInstance.SDKVersion()); err != nil {
		t.Errorf("SendMulticastDryRun() = %v", err)
	}
}

func TestSendMulticastPartialFailure(t *testing.T) {
	success := []fcmResponse{
		testSuccessResponse[0], // Only one success
	}

	var reqBody, respBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqBody, _ = ioutil.ReadAll(r.Body)
		w.Header().Set("Content-Type", wantMime)
		w.Write(respBody)
	}))
	defer ts.Close()

	ctx := context.Background()
	appInstance := newTestMessagingAppForBatch(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}
	client.fcmClient.batchEndpoint = ts.URL

	for idx, tc := range httpErrors {
		failures := []string{tc.resp} // This will be the second part of the multipart response
		respBody, err = createMultipartResponse(success, failures)
		if err != nil {
			t.Fatalf("[%d] Failed to create multipart response: %v", idx, err)
		}

		br, err := client.SendMulticast(ctx, testMulticastMessage) // testMulticastMessage has 2 tokens
		if err != nil {
			t.Fatalf("[%d] SendMulticast() returned error: %v", idx, err)
		}

		if err := checkPartialErrorBatchResponse(br, tc); err != nil { // checkPartialErrorBatchResponse expects 1 success, 1 failure
			t.Errorf("[%d] SendMulticast() batch response check failed: %v", idx, err)
		}
		// checkMultipartRequest is not directly applicable here in the same way as SendAll
		// because SendMulticast translates to multiple single sends if SendEach path is taken,
		// or one batch if SendAll path is taken.
		// The refactored SendMulticast uses SendAll, so reqBody here *is* the batch request.
		if err := checkMultipartRequest(reqBody, false, appInstance.SDKVersion()); err != nil {
			t.Errorf("[%d] MultipartRequest check failed: %v", idx, err)
		}
	}
}


func checkSuccessfulBatchResponseForSendEach(br *BatchResponse, dryRun bool) error {
	if br.SuccessCount != 2 {
		return fmt.Errorf("SuccessCount = %d; want = 2", br.SuccessCount)
	}
	if br.FailureCount != 0 {
		return fmt.Errorf("FailureCount = %d; want = 0", br.FailureCount)
	}
	if len(br.Responses) != 2 {
		return fmt.Errorf("len(Responses) = %d; want = 2", len(br.Responses))
	}

	for idx, r := range br.Responses {
		// Use the correct project ID from testSuccessResponse for comparison
		if err := checkSuccessfulSendResponse(r, testSuccessResponse[idx].Name); err != nil {
			return fmt.Errorf("Responses[%d]: %v", idx, err)
		}
	}
	// No single batch request body to check for SendEach
	return nil
}


func checkSuccessfulBatchResponse(br *BatchResponse, req []byte, dryRun bool, sdkVersion string) error {
	if br.SuccessCount != 2 {
		return fmt.Errorf("SuccessCount = %d; want = 2", br.SuccessCount)
	}
	if br.FailureCount != 0 {
		return fmt.Errorf("FailureCount = %d; want = 0", br.FailureCount)
	}
	if len(br.Responses) != 2 {
		return fmt.Errorf("len(Responses) = %d; want = 2", len(br.Responses))
	}

	for idx, r := range br.Responses {
		if err := checkSuccessfulSendResponse(r, testSuccessResponse[idx].Name); err != nil {
			return fmt.Errorf("Responses[%d]: %v", idx, err)
		}
	}

	if err := checkMultipartRequest(req, dryRun, sdkVersion); err != nil {
		return fmt.Errorf("MultipartRequest: %v", err)
	}

	return nil
}

func checkTotalErrorBatchResponse(br *BatchResponse, tc struct {
	resp, want string
	check      func(error) bool
}) error {
	if br.SuccessCount != 0 {
		return fmt.Errorf("SuccessCount = %d; want = 0", br.SuccessCount)
	}
	// For SendEach, each message results in a response. If testMessages has 2 items, FailureCount should be 2.
	expectedFailures := len(testMessages)
	if br.FailureCount != expectedFailures {
		return fmt.Errorf("FailureCount = %d; want = %d", br.FailureCount, expectedFailures)
	}
	if len(br.Responses) != expectedFailures {
		return fmt.Errorf("len(Responses) = %d; want = %d", len(br.Responses), expectedFailures)
	}


	for i, r := range br.Responses {
		if r.Success {
			return fmt.Errorf("Responses[%d]: Success = true; want = false", i)
		}
		if r.Error == nil || r.Error.Error() != tc.want || !tc.check(r.Error) {
			return fmt.Errorf("Responses[%d]: Error = %v; want = %q", i, r.Error, tc.want)
		}
		if r.MessageID != "" {
			return fmt.Errorf("Responses[%d]: MessageID = %q; want = %q", i, r.MessageID, "")
		}
	}

	return nil
}

func checkPartialErrorBatchResponse(br *BatchResponse, tc struct {
	resp, want string
	check      func(error) bool
}) error {
	// This function assumes a specific scenario of 1 success and 1 failure for a 2-message batch.
	if br.SuccessCount != 1 {
		return fmt.Errorf("SuccessCount = %d; want = 1", br.SuccessCount)
	}
	if br.FailureCount != 1 {
		return fmt.Errorf("FailureCount = %d; want = 1", br.FailureCount)
	}
	if len(br.Responses) != 2 { // Assuming testMessages or equivalent always has 2 items for these tests
		return fmt.Errorf("len(Responses) = %d; want = 2", len(br.Responses))
	}

	// Assuming the first response is success and second is failure for this helper
	if err := checkSuccessfulSendResponse(br.Responses[0], testSuccessResponse[0].Name); err != nil {
		return fmt.Errorf("Responses[0]: %v", err)
	}

	r := br.Responses[1]
	if r.Success {
		return fmt.Errorf("Responses[1]: Success = true; want = false")
	}
	if r.Error == nil || r.Error.Error() != tc.want || !tc.check(r.Error) {
		return fmt.Errorf("Responses[1]: Error = %v; want = %q", r.Error, tc.want)
	}
	if r.MessageID != "" {
		return fmt.Errorf("Responses[1]: MessageID = %q; want = %q", r.MessageID, "")
	}

	return nil
}

func checkSuccessfulSendResponse(r *SendResponse, wantID string) error {
	if !r.Success {
		return fmt.Errorf("Success = false; want = true")
	}
	if r.Error != nil {
		return fmt.Errorf("Error = %v; want = nil", r.Error)
	}
	if r.MessageID != wantID {
		return fmt.Errorf("MessageID = %q; want = %q", r.MessageID, wantID)
	}
	return nil
}

func checkMultipartRequest(b []byte, dryRun bool, sdkVersion string) error {
	reader := multipart.NewReader(bytes.NewBuffer((b)), multipartBoundary)
	count := 0
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		if err := checkRequestPart(part, dryRun, sdkVersion); err != nil {
			return fmt.Errorf("[%d] %v", count, err)
		}
		count++
	}

	if count != 2 { // Assuming testMessages always has 2 items for these batch tests
		return fmt.Errorf("PartsCount = %d; want = 2", count)
	}
	return nil
}

func checkRequestPart(part *multipart.Part, dryRun bool, sdkVersion string) error {
	r, err := http.ReadRequest(bufio.NewReader(part))
	if err != nil {
		return err
	}

	if r.Method != http.MethodPost {
		return fmt.Errorf("Method = %q; want = %q", r.Method, http.MethodPost)
	}
	if r.RequestURI != wantSendURL { // wantSendURL uses testMessagingProjectID
		return fmt.Errorf("URL = %q; want = %q", r.RequestURI, wantSendURL)
	}
	if h := r.Header.Get("X-GOOG-API-FORMAT-VERSION"); h != "2" {
		return fmt.Errorf("X-GOOG-API-FORMAT-VERSION = %q; want = %q", h, "2")
	}

	clientVersion := "fire-admin-go/" + sdkVersion // Use passed sdkVersion
	if h := r.Header.Get("X-FIREBASE-CLIENT"); h != clientVersion {
		return fmt.Errorf("X-FIREBASE-CLIENT = %q; want = %q", h, clientVersion)
	}

	bodyBytes, _ := ioutil.ReadAll(r.Body)
	var parsed map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &parsed); err != nil {
		return err
	}

	if _, ok := parsed["message"]; !ok {
		return fmt.Errorf("Invalid message body = %v", parsed)
	}

	validate, ok := parsed["validate_only"]
	if dryRun {
		if !ok || validate != true {
			return fmt.Errorf("ValidateOnly = %v; want = true", validate)
		}
	} else if ok {
		return fmt.Errorf("ValidateOnly = %v; want none", validate)
	}

	return nil
}

func createMultipartResponse(success []fcmResponse, failure []string) ([]byte, error) {
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	writer.SetBoundary(multipartBoundary)
	for idx, data := range success {
		b, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}

		var partBuffer bytes.Buffer
		partBuffer.WriteString("HTTP/1.1 200 OK\r\n")
		partBuffer.WriteString("Content-Type: application/json\r\n\r\n")
		partBuffer.Write(b)

		if err := writeResponsePart(writer, partBuffer.Bytes(), idx); err != nil {
			return nil, err
		}
	}

	for idx, data := range failure {
		var partBuffer bytes.Buffer
		partBuffer.WriteString("HTTP/1.1 500 Internal Server Error\r\n")
		partBuffer.WriteString("Content-Type: application/json\r\n\r\n")
		partBuffer.WriteString(data)

		if err := writeResponsePart(writer, partBuffer.Bytes(), idx+len(success)); err != nil {
			return nil, err
		}
	}

	writer.Close()
	return buffer.Bytes(), nil
}

func writeResponsePart(writer *multipart.Writer, data []byte, idx int) error {
	header := make(textproto.MIMEHeader)
	header.Add("Content-Type", "application/http")
	header.Add("Content-Id", fmt.Sprintf("%d", idx+1))
	part, err := writer.CreatePart(header)
	if err != nil {
		return err
	}

	_, err = part.Write(data)
	return err
}
