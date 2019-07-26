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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

const wantMime = "multipart/mixed; boundary=__END_OF_PART__"

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
	for i := 0; i < 101; i++ {
		messages = append(messages, &Message{Topic: "test-topic"})
	}

	want := "messages must not contain more than 100 elements"
	br, err := client.SendAll(ctx, messages)
	if err == nil || err.Error() != want {
		t.Errorf("SendAll(nil) = (%v, %v); want = (nil, %q)", br, err, want)
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
		t.Errorf("SendAll(nil) = (%v, %v); want = (nil, %q)", br, err, want)
	}
}

func TestSendAll(t *testing.T) {
	success := []fcmResponse{
		{
			Name: "projects/test-project/messages/1",
		},
		{
			Name: "projects/test-project/messages/2",
		},
	}
	resp, err := createMultipartResponse(success, nil)
	if err != nil {
		t.Fatal(err)
	}

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

	br, err := client.SendAll(ctx, testMessages)
	if err != nil {
		t.Fatal(err)
	}

	if br.SuccessCount != 2 {
		t.Errorf("SuccessCount = %d; want = 2", br.SuccessCount)
	}
	if br.FailureCount != 0 {
		t.Errorf("FailureCount = %d; want = 0", br.FailureCount)
	}
	if len(br.Responses) != 2 {
		t.Errorf("len(Responses) = %d; want = 2", len(br.Responses))
	}

	for idx, r := range br.Responses {
		if err := checkSuccessfulSendResponse(r, success[idx].Name); err != nil {
			t.Errorf("Responses[%d]: %v", idx, err)
		}
	}
}

func TestSendAllPartialFailure(t *testing.T) {
	success := []fcmResponse{
		{
			Name: "projects/test-project/messages/1",
		},
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

	for _, tc := range httpErrors {
		failures := []string{tc.resp}
		resp, err = createMultipartResponse(success, failures)
		if err != nil {
			t.Fatal(err)
		}

		br, err := client.SendAll(ctx, testMessages)
		if err != nil {
			t.Fatal(err)
		}

		if br.SuccessCount != 1 {
			t.Errorf("SuccessCount = %d; want = 1", br.SuccessCount)
		}
		if br.FailureCount != 1 {
			t.Errorf("FailureCount = %d; want = 1", br.FailureCount)
		}
		if len(br.Responses) != 2 {
			t.Errorf("len(Responses) =%d; want = 2", len(br.Responses))
		}

		if err := checkSuccessfulSendResponse(br.Responses[0], success[0].Name); err != nil {
			t.Errorf("Responses[0]: %v", err)
		}

		r := br.Responses[1]
		if r.Success {
			t.Errorf("Responses[1]: Success = true; want = false")
		}
		if r.Error == nil || r.Error.Error() != tc.want || !tc.check(r.Error) {
			t.Errorf("Responses[1]: Error = %v; want = %q", r.Error, tc.want)
		}
		if r.MessageID != "" {
			t.Errorf("Responses[1]: MessageID = %q; want = %q", r.MessageID, "")
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
	client.client.RetryConfig = nil

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

	_, err = client.SendAll(ctx, testMessages)
	if err == nil {
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

	_, err = client.SendAll(ctx, testMessages)
	if err == nil {
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

	_, err = client.SendAll(ctx, testMessages)
	if err == nil {
		t.Fatal("SendAll() = nil; want = error")
	}
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
