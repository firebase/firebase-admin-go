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
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"sync"
	"testing"

	"google.golang.org/api/option"
)

var testMessages = []*Message{
	{Topic: "topic1"},
	{Topic: "topic2"},
}
var testMulticastMessage = &MulticastMessage{
	Tokens: []string{"token1", "token2"},
}
var testSuccessResponse = []fcmResponse{
	{
		Name: "projects/test-project/messages/1",
	},
	{
		Name: "projects/test-project/messages/2",
	},
}

const wantMime = "multipart/mixed; boundary=__END_OF_PART__"
const wantSendURL = "/v1/projects/test-project/messages:send"

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

func TestSendEachWorkerPoolScenarios(t *testing.T) {
	scenarios := []struct {
		name         string
		numMessages  int
		// numWorkers is now fixed at 50 in sendEachInBatch. This comment is for context.
		// We will test different loads relative to this fixed size.
		allSuccessful bool
		testNameSuffix string // To make test names more descriptive if needed
	}{
		{numMessages: 5, allSuccessful: true, testNameSuffix: " (5msg < 50workers)"},
		{numMessages: 50, allSuccessful: true, testNameSuffix: " (50msg == 50workers)"},
		{numMessages: 75, allSuccessful: true, testNameSuffix: " (75msg > 50workers)"},
		{numMessages: 75, allSuccessful: false, testNameSuffix: " (75msg > 50workers, with Failures)"},
	}

	for _, s := range scenarios {
		scenarioName := fmt.Sprintf("NumMessages_%d_AllSuccess_%v%s", s.numMessages, s.allSuccessful, s.testNameSuffix)
		t.Run(scenarioName, func(t *testing.T) {
			ctx := context.Background()
			client, err := NewClient(ctx, testMessagingConfig)
			if err != nil {
				t.Fatal(err)
			}

			messages := make([]*Message, s.numMessages)
			expectedSuccessCount := s.numMessages
			expectedFailureCount := 0

			serverHitCount := 0
			mu := &sync.Mutex{} // To protect serverHitCount

			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				serverHitCount++
				currentHit := serverHitCount // Capture current hit for stable value in response
				mu.Unlock()

				var reqBody fcmRequest
				if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				
				// For "Messages > Workers with Failures", make every 3rd message fail
				if !s.allSuccessful && currentHit%3 == 0 {
					w.WriteHeader(http.StatusInternalServerError)
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(map[string]interface{}{
						"error": map[string]interface{}{
							"message": "Simulated server error",
							"status":  "INTERNAL",
						},
					})
				} else {
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(map[string]string{
						"name": fmt.Sprintf("projects/test-project/messages/%s-%d", reqBody.Message.Topic, currentHit),
					})
				}
			}))
			defer ts.Close()
			client.fcmEndpoint = ts.URL

			for i := 0; i < s.numMessages; i++ {
				messages[i] = &Message{Topic: fmt.Sprintf("topic%d", i)}
			}
			
			if !s.allSuccessful {
				expectedSuccessCount = 0
				expectedFailureCount = 0
				for i := 0; i < s.numMessages; i++ {
					if (i+1)%3 == 0 { // Matches server logic for failures (1-indexed hit count)
						expectedFailureCount++
					} else {
						expectedSuccessCount++
					}
				}
			}


			br, err := client.SendEach(ctx, messages)
			if err != nil {
				t.Fatalf("SendEach() unexpected error: %v", err)
			}

			if br.SuccessCount != expectedSuccessCount {
				t.Errorf("SuccessCount = %d; want = %d", br.SuccessCount, expectedSuccessCount)
			}
			if br.FailureCount != expectedFailureCount {
				t.Errorf("FailureCount = %d; want = %d", br.FailureCount, expectedFailureCount)
			}
			if len(br.Responses) != s.numMessages {
				t.Errorf("len(Responses) = %d; want = %d", len(br.Responses), s.numMessages)
			}
			mu.Lock() // Protect serverHitCount read
			if serverHitCount != s.numMessages {
				t.Errorf("Server hit count = %d; want = %d", serverHitCount, s.numMessages)
			}
			mu.Unlock()

			for i, resp := range br.Responses {
				isExpectedToSucceed := s.allSuccessful || (i+1)%3 != 0
				if resp.Success != isExpectedToSucceed {
					t.Errorf("Responses[%d].Success = %v; want = %v", i, resp.Success, isExpectedToSucceed)
				}
				if isExpectedToSucceed && resp.MessageID == "" {
					t.Errorf("Responses[%d].MessageID is empty for a successful message", i)
				}
				if !isExpectedToSucceed && resp.Error == nil {
					t.Errorf("Responses[%d].Error is nil for a failed message", i)
				}
			}
		})
	}
}

func TestSendEachResponseOrderWithConcurrency(t *testing.T) {
	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}

	numMessages := 75 // Ensure this is > new worker count of 50
	messages := make([]*Message, numMessages)
	for i := 0; i < numMessages; i++ {
		messages[i] = &Message{Token: fmt.Sprintf("token%d", i)} // Using Token for unique identification
	}

	// serverHitCount and messageIDLog are protected by mu
	serverHitCount := 0
	messageIDLog := make(map[string]int) // Maps message identifier (token) to hit order
	var mu sync.Mutex

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		serverHitCount++
		hitOrder := serverHitCount
		mu.Unlock()

		var reqBody fcmRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		messageIdentifier := reqBody.Message.Token // Assuming token is unique and part of the request

		mu.Lock()
		messageIDLog[messageIdentifier] = hitOrder // Log which message (by token) was processed in which hit order
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		// Construct message ID that includes the original token to verify later
		json.NewEncoder(w).Encode(map[string]string{
			"name": fmt.Sprintf("projects/test-project/messages/msg_for_%s", messageIdentifier),
		})
	}))
	defer ts.Close()
	client.fcmEndpoint = ts.URL

	br, err := client.SendEach(ctx, messages)
	if err != nil {
		t.Fatalf("SendEach() unexpected error: %v", err)
	}

	if br.SuccessCount != numMessages {
		t.Errorf("SuccessCount = %d; want = %d", br.SuccessCount, numMessages)
	}
	if len(br.Responses) != numMessages {
		t.Errorf("len(Responses) = %d; want = %d", len(br.Responses), numMessages)
	}

	if serverHitCount != numMessages {
		t.Errorf("Server hit count = %d; want = %d", serverHitCount, numMessages)
	}

	for i, resp := range br.Responses {
		if !resp.Success {
			t.Errorf("Responses[%d] was not successful: %v", i, resp.Error)
			continue
		}
		expectedMessageIDPart := fmt.Sprintf("msg_for_token%d", i)
		if !strings.Contains(resp.MessageID, expectedMessageIDPart) {
			t.Errorf("Responses[%d].MessageID = %q; want to contain %q", i, resp.MessageID, expectedMessageIDPart)
		}
	}

	// This test doesn't directly check if message N was processed by worker X,
	// but it ensures that all messages are processed and their responses are correctly ordered.
	// The messageIDLog could be used for more detailed analysis of concurrency if needed,
	// but for now, ensuring correct final order and all messages processed is the key.
}

func TestSendEachEarlyValidationSkipsSend(t *testing.T) {
	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}

	messagesWithInvalid := []*Message{
		{Topic: "topic1"},
		nil, // Invalid message
		{Topic: "topic2"},
	}

	serverHitCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverHitCount++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{ "name":"projects/test-project/messages/1" }`))
	}))
	defer ts.Close()
	client.fcmEndpoint = ts.URL

	br, err := client.SendEach(ctx, messagesWithInvalid)
	if err == nil {
		t.Errorf("SendEach() expected error for invalid message, got nil")
	}
	if br != nil {
		t.Errorf("SendEach() expected nil BatchResponse for invalid message, got %v", br)
	}

	if serverHitCount != 0 {
		t.Errorf("Server hit count = %d; want = 0 due to early validation failure", serverHitCount)
	}

	// Test with invalid message at the beginning
	messagesWithInvalidFirst := []*Message{
		{Topic: "invalid", Condition: "invalid"}, // Invalid: both Topic and Condition
		{Topic: "topic1"},
	}
	serverHitCount = 0
	br, err = client.SendEach(ctx, messagesWithInvalidFirst)
	if err == nil {
		t.Errorf("SendEach() expected error for invalid first message, got nil")
	}
	if br != nil {
		t.Errorf("SendEach() expected nil BatchResponse for invalid first message, got %v", br)
	}
	if serverHitCount != 0 {
		t.Errorf("Server hit count = %d; want = 0 for invalid first message", serverHitCount)
	}

	// Test with invalid message at the end
	messagesWithInvalidLast := []*Message{
		{Topic: "topic1"},
		{Token: "test-token", Data: map[string]string{"key": string(make([]byte, 4097))}}, // Invalid: data payload too large
	}
	serverHitCount = 0
	br, err = client.SendEach(ctx, messagesWithInvalidLast)
	if err == nil {
		t.Errorf("SendEach() expected error for invalid last message, got nil")
	}
	if br != nil {
		t.Errorf("SendEach() expected nil BatchResponse for invalid last message, got %v", br)
	}
	if serverHitCount != 0 {
		t.Errorf("Server hit count = %d; want = 0 for invalid last message", serverHitCount)
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
	client, err := NewClient(ctx, testMessagingConfig)
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
	client, err := NewClient(ctx, testMessagingConfig)
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
	client, err := NewClient(ctx, testMessagingConfig)
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
				w.Write([]byte("{ \"name\":\"" + testSuccessResponse[idx].Name + "\" }"))
			}
		}
	}))
	defer ts.Close()
	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.fcmEndpoint = ts.URL

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
				w.Write([]byte("{ \"name\":\"" + testSuccessResponse[idx].Name + "\" }"))
			}
		}
	}))
	defer ts.Close()
	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.fcmEndpoint = ts.URL

	br, err := client.SendEachDryRun(ctx, testMessages)
	if err != nil {
		t.Fatal(err)
	}

	if err := checkSuccessfulBatchResponseForSendEach(br, true); err != nil {
		t.Errorf("SendEach() = %v", err)
	}
}

func TestSendEachPartialFailure(t *testing.T) {
	success := []fcmResponse{
		{
			Name: "projects/test-project/messages/1",
		},
	}

	var failures []string
	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}

	for idx, tc := range httpErrors {
		failures = []string{tc.resp} // tc.resp is the error JSON string
		serverHitCount := 0
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			serverHitCount++
			reqBody, _ := ioutil.ReadAll(r.Body)
			var msgIn fcmRequest
			json.Unmarshal(reqBody, &msgIn)

			// Respond successfully for the first message (topic1)
			if msgIn.Message.Topic == testMessages[0].Topic {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{ "name":"` + success[0].Name + `" }`))
			} else if msgIn.Message.Topic == testMessages[1].Topic { // Respond with error for the second message (topic2)
				w.WriteHeader(http.StatusInternalServerError)
				w.Header().Set("Content-Type", "application/json") // Errors are also JSON
				w.Write([]byte(failures[0]))
			} else {
				// Should not happen with current testMessages
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error":"unknown topic"}`))
			}
		}))
		defer ts.Close()
		client.fcmEndpoint = ts.URL

		br, err := client.SendEach(ctx, testMessages) // testMessages has 2 messages
		if err != nil {
			t.Fatalf("[%d] SendEach() unexpected error: %v", idx, err)
		}

		if serverHitCount != len(testMessages) {
			t.Errorf("[%d] Server hit count = %d; want = %d", idx, serverHitCount, len(testMessages))
		}

		if err := checkPartialErrorBatchResponse(br, tc); err != nil {
			t.Errorf("[%d] SendEach() = %v", idx, err)
		}
	}
}

func TestSendEachTotalFailure(t *testing.T) {
	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.fcmClient.httpClient.RetryConfig = nil

	for idx, tc := range httpErrors {
		serverHitCount := 0
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			serverHitCount++
			w.WriteHeader(http.StatusInternalServerError)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(tc.resp)) // tc.resp is the error JSON string
		}))
		defer ts.Close()
		client.fcmEndpoint = ts.URL

		br, err := client.SendEach(ctx, testMessages) // testMessages has 2 messages
		if err != nil {
			t.Fatalf("[%d] SendEach() unexpected error: %v", idx, err)
		}

		if serverHitCount != len(testMessages) {
			t.Errorf("[%d] Server hit count = %d; want = %d", idx, serverHitCount, len(testMessages))
		}

		if err := checkTotalErrorBatchResponse(br, tc); err != nil {
			t.Errorf("[%d] SendEach() = %v", idx, err)
		}
	}
}

func TestSendEachForMulticastNil(t *testing.T) {
	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
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
		t.Errorf("SendEachForMulticast(nil) = (%v, %v); want = (nil, %q)", br, err, want)
	}
}

func TestSendEachForMulticastEmptyArray(t *testing.T) {
	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
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
	client, err := NewClient(ctx, testMessagingConfig)
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
	client, err := NewClient(ctx, testMessagingConfig)
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
				w.Write([]byte("{ \"name\":\"" + testSuccessResponse[idx].Name + "\" }"))
			}
		}
	}))
	defer ts.Close()
	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.fcmEndpoint = ts.URL

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
				w.Write([]byte("{ \"name\":\"" + testSuccessResponse[idx].Name + "\" }"))
			}
		}
	}))
	defer ts.Close()

	ctx := context.Background()

	conf := *testMessagingConfig
	optEndpoint := option.WithEndpoint(ts.URL)
	conf.Opts = append(conf.Opts, optEndpoint)

	client, err := NewClient(ctx, &conf)
	if err != nil {
		t.Fatal(err)
	}

	if ts.URL != client.fcmEndpoint {
		t.Errorf("client.fcmEndpoint = %q; want = %q", client.fcmEndpoint, ts.URL)
	}

	br, err := client.SendEachForMulticast(ctx, testMulticastMessage)
	if err := checkSuccessfulBatchResponseForSendEach(br, false); err != nil {
		t.Errorf("SendEachForMulticast() = %v", err)
	}
}

func TestSendEachForMulticastDryRun(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, _ := ioutil.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		for idx, token := range testMulticastMessage.Tokens {
			if strings.Contains(string(req), token) {
				w.Write([]byte("{ \"name\":\"" + testSuccessResponse[idx].Name + "\" }"))
			}
		}
	}))
	defer ts.Close()
	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.fcmEndpoint = ts.URL

	br, err := client.SendEachForMulticastDryRun(ctx, testMulticastMessage)
	if err != nil {
		t.Fatal(err)
	}

	if err := checkSuccessfulBatchResponseForSendEach(br, true); err != nil {
		t.Errorf("SendEachForMulticastDryRun() = %v", err)
	}
}

func TestSendEachForMulticastPartialFailure(t *testing.T) {
	success := []fcmResponse{
		{
			Name: "projects/test-project/messages/1",
		},
	}

	var failures []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, _ := ioutil.ReadAll(r.Body)

		for idx, token := range testMulticastMessage.Tokens {
			if strings.Contains(string(req), token) {
				// Write success for token1 and error for token2
				if idx%2 == 0 {
					w.Header().Set("Content-Type", wantMime)
					w.Write([]byte("{ \"name\":\"" + success[0].Name + "\" }"))
				} else {
					w.WriteHeader(http.StatusInternalServerError)
					w.Header().Set("Content-Type", wantMime)
					w.Write([]byte(failures[0]))
				}
			}
		}
	}))
	defer ts.Close()

	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.fcmEndpoint = ts.URL

	for idx, tc := range httpErrors {
		failures = []string{tc.resp}

		br, err := client.SendEachForMulticast(ctx, testMulticastMessage)
		if err != nil {
			t.Fatal(err)
		}

		if err := checkPartialErrorBatchResponse(br, tc); err != nil {
			t.Errorf("[%d] SendEachForMulticast() = %v", idx, err)
		}
	}
}

func TestSendAllEmptyArray(t *testing.T) {
	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
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
		t.Errorf("SendAll(nil) = (%v, %v); want = (nil, %q)", br, err, want)
	}
}

func TestSendAllTooManyMessages(t *testing.T) {
	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
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
	client, err := NewClient(ctx, testMessagingConfig)
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
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.batchEndpoint = ts.URL

	br, err := client.SendAll(ctx, testMessages)
	if err != nil {
		t.Fatal(err)
	}

	if err := checkSuccessfulBatchResponse(br, req, false); err != nil {
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
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.batchEndpoint = ts.URL

	br, err := client.SendAllDryRun(ctx, testMessages)
	if err != nil {
		t.Fatal(err)
	}

	if err := checkSuccessfulBatchResponse(br, req, true); err != nil {
		t.Errorf("SendAll() = %v", err)
	}
}

func TestSendAllPartialFailure(t *testing.T) {
	success := []fcmResponse{
		{
			Name: "projects/test-project/messages/1",
		},
	}

	var req, resp []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, _ = ioutil.ReadAll(r.Body)
		w.Header().Set("Content-Type", wantMime)
		w.Write(resp)
	}))
	defer ts.Close()

	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.batchEndpoint = ts.URL

	for idx, tc := range httpErrors {
		failures := []string{tc.resp}
		resp, err = createMultipartResponse(success, failures)
		if err != nil {
			t.Fatal(err)
		}

		br, err := client.SendAll(ctx, testMessages)
		if err != nil {
			t.Fatal(err)
		}

		if err := checkPartialErrorBatchResponse(br, tc); err != nil {
			t.Errorf("[%d] SendAll() = %v", idx, err)
		}

		if err := checkMultipartRequest(req, false); err != nil {
			t.Errorf("[%d] MultipartRequest: %v", idx, err)
		}
	}
}

func TestSendAllTotalFailure(t *testing.T) {
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
	client.batchEndpoint = ts.URL
	client.fcmClient.httpClient.RetryConfig = nil

	for _, tc := range httpErrors {
		resp = tc.resp
		br, err := client.SendAll(ctx, []*Message{{Topic: "topic"}})
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
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.batchEndpoint = ts.URL
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
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.batchEndpoint = ts.URL
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
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.batchEndpoint = ts.URL
	if _, err = client.SendAll(ctx, testMessages); err == nil {
		t.Fatal("SendAll() = nil; want = error")
	}
}

func TestSendMulticastNil(t *testing.T) {
	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
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
		t.Errorf("SendMulticast(nil) = (%v, %v); want = (nil, %q)", br, err, want)
	}
}

func TestSendMulticastEmptyArray(t *testing.T) {
	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
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
	client, err := NewClient(ctx, testMessagingConfig)
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
	client, err := NewClient(ctx, testMessagingConfig)
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
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.batchEndpoint = ts.URL

	br, err := client.SendMulticast(ctx, testMulticastMessage)
	if err != nil {
		t.Fatal(err)
	}

	if err := checkSuccessfulBatchResponse(br, req, false); err != nil {
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

	conf := *testMessagingConfig
	customBatchEndpoint := fmt.Sprintf("%s/v1", ts.URL)
	optEndpoint := option.WithEndpoint(customBatchEndpoint)
	conf.Opts = append(conf.Opts, optEndpoint)

	client, err := NewClient(ctx, &conf)
	if err != nil {
		t.Fatal(err)
	}

	if customBatchEndpoint != client.batchEndpoint {
		t.Errorf("client.batchEndpoint = %q; want = %q", client.batchEndpoint, customBatchEndpoint)
	}

	br, err := client.SendMulticast(ctx, testMulticastMessage)
	if err != nil {
		t.Fatal(err)
	}

	if err := checkSuccessfulBatchResponse(br, req, false); err != nil {
		t.Errorf("SendMulticast() = %v", err)
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
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.batchEndpoint = ts.URL

	br, err := client.SendMulticastDryRun(ctx, testMulticastMessage)
	if err != nil {
		t.Fatal(err)
	}

	if err := checkSuccessfulBatchResponse(br, req, true); err != nil {
		t.Errorf("SendMulticastDryRun() = %v", err)
	}
}

func TestSendMulticastPartialFailure(t *testing.T) {
	success := []fcmResponse{
		testSuccessResponse[0],
	}

	var resp []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", wantMime)
		w.Write(resp)
	}))
	defer ts.Close()

	ctx := context.Background()
	client, err := NewClient(ctx, testMessagingConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.batchEndpoint = ts.URL

	for idx, tc := range httpErrors {
		failures := []string{tc.resp}
		resp, err = createMultipartResponse(success, failures)
		if err != nil {
			t.Fatal(err)
		}

		br, err := client.SendMulticast(ctx, testMulticastMessage)
		if err != nil {
			t.Fatal(err)
		}

		if err := checkPartialErrorBatchResponse(br, tc); err != nil {
			t.Errorf("[%d] SendMulticast() = %v", idx, err)
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
		if err := checkSuccessfulSendResponse(r, testSuccessResponse[idx].Name); err != nil {
			return fmt.Errorf("Responses[%d]: %v", idx, err)
		}
	}

	return nil
}

func checkSuccessfulBatchResponse(br *BatchResponse, req []byte, dryRun bool) error {
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

	if err := checkMultipartRequest(req, dryRun); err != nil {
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
	if br.FailureCount != 2 {
		return fmt.Errorf("FailureCount = %d; want = 2", br.FailureCount)
	}
	if len(br.Responses) != 2 {
		return fmt.Errorf("len(Responses) = %d; want = 2", len(br.Responses))
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
	if br.SuccessCount != 1 {
		return fmt.Errorf("SuccessCount = %d; want = 1", br.SuccessCount)
	}
	if br.FailureCount != 1 {
		return fmt.Errorf("FailureCount = %d; want = 1", br.FailureCount)
	}
	if len(br.Responses) != 2 {
		return fmt.Errorf("len(Responses) = %d; want = 2", len(br.Responses))
	}

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

func checkMultipartRequest(b []byte, dryRun bool) error {
	reader := multipart.NewReader(bytes.NewBuffer((b)), multipartBoundary)
	count := 0
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		if err := checkRequestPart(part, dryRun); err != nil {
			return fmt.Errorf("[%d] %v", count, err)
		}
		count++
	}

	if count != 2 {
		return fmt.Errorf("PartsCount = %d; want = 2", count)
	}
	return nil
}

func checkRequestPart(part *multipart.Part, dryRun bool) error {
	r, err := http.ReadRequest(bufio.NewReader(part))
	if err != nil {
		return err
	}

	if r.Method != http.MethodPost {
		return fmt.Errorf("Method = %q; want = %q", r.Method, http.MethodPost)
	}
	if r.RequestURI != wantSendURL {
		return fmt.Errorf("URL = %q; want = %q", r.RequestURI, wantSendURL)
	}
	if h := r.Header.Get("X-GOOG-API-FORMAT-VERSION"); h != "2" {
		return fmt.Errorf("X-GOOG-API-FORMAT-VERSION = %q; want = %q", h, "2")
	}

	clientVersion := "fire-admin-go/" + testMessagingConfig.Version
	if h := r.Header.Get("X-FIREBASE-CLIENT"); h != clientVersion {
		return fmt.Errorf("X-FIREBASE-CLIENT = %q; want = %q", h, clientVersion)
	}

	b, _ := ioutil.ReadAll(r.Body)
	var parsed map[string]interface{}
	if err := json.Unmarshal(b, &parsed); err != nil {
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
