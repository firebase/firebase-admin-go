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
	"testing"
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
