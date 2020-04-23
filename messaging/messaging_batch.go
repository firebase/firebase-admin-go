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
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"

	"firebase.google.com/go/internal"
)

const maxMessages = 500
const multipartBoundary = "__END_OF_PART__"

// MulticastMessage represents a message that can be sent to multiple devices via Firebase Cloud
// Messaging (FCM).
//
// It contains payload information as well as the list of device registration tokens to which the
// message should be sent. A single MulticastMessage may contain up to 500 registration tokens.
type MulticastMessage struct {
	Tokens       []string
	Data         map[string]string
	Notification *Notification
	Android      *AndroidConfig
	Webpush      *WebpushConfig
	APNS         *APNSConfig
}

func (mm *MulticastMessage) toMessages() ([]*Message, error) {
	if len(mm.Tokens) == 0 {
		return nil, errors.New("tokens must not be nil or empty")
	}
	if len(mm.Tokens) > maxMessages {
		return nil, fmt.Errorf("tokens must not contain more than %d elements", maxMessages)
	}

	var messages []*Message
	for _, token := range mm.Tokens {
		temp := &Message{
			Token:        token,
			Data:         mm.Data,
			Notification: mm.Notification,
			Android:      mm.Android,
			Webpush:      mm.Webpush,
			APNS:         mm.APNS,
		}
		messages = append(messages, temp)
	}

	return messages, nil
}

// SendResponse represents the status of an individual message that was sent as part of a batch
// request.
type SendResponse struct {
	Success   bool
	MessageID string
	Error     error
}

// BatchResponse represents the response from the `SendAll()` and `SendMulticast()` APIs.
type BatchResponse struct {
	SuccessCount int
	FailureCount int
	Responses    []*SendResponse
}

// SendAll sends the messages in the given array via Firebase Cloud Messaging.
//
// The messages array may contain up to 500 messages. SendAll employs batching to send the entire
// array of mssages as a single RPC call. Compared to the `Send()` function,
// this is a significantly more efficient way to send multiple messages. The responses list
// obtained from the return value corresponds to the order of the input messages. An error from
// SendAll indicates a total failure -- i.e. none of the messages in the array could be sent.
// Partial failures are indicated by a `BatchResponse` return value.
func (c *fcmClient) SendAll(ctx context.Context, messages []*Message) (*BatchResponse, error) {
	return c.sendBatch(ctx, messages, false)
}

// SendAllDryRun sends the messages in the given array via Firebase Cloud Messaging in the
// dry run (validation only) mode.
//
// This function does not actually deliver any messages to target devices. Instead, it performs all
// the SDK-level and backend validations on the messages, and emulates the send operation.
//
// The messages array may contain up to 500 messages. SendAllDryRun employs batching to send the
// entire array of mssages as a single RPC call. Compared to the `SendDryRun()` function, this
// is a significantly more efficient way to validate sending multiple messages. The responses list
// obtained from the return value corresponds to the order of the input messages. An error from
// SendAllDryRun indicates a total failure -- i.e. none of the messages in the array could be sent
// for validation. Partial failures are indicated by a `BatchResponse` return value.
func (c *fcmClient) SendAllDryRun(ctx context.Context, messages []*Message) (*BatchResponse, error) {
	return c.sendBatch(ctx, messages, true)
}

// SendMulticast sends the given multicast message to all the FCM registration tokens specified.
//
// The tokens array in MulticastMessage may contain up to 500 tokens. SendMulticast uses the
// `SendAll()` function to send the given message to all the target recipients. The
// responses list obtained from the return value corresponds to the order of the input tokens. An
// error from SendMulticast indicates a total failure -- i.e. the message could not be sent to any
// of the recipients. Partial failures are indicated by a `BatchResponse` return value.
func (c *fcmClient) SendMulticast(ctx context.Context, message *MulticastMessage) (*BatchResponse, error) {
	messages, err := toMessages(message)
	if err != nil {
		return nil, err
	}

	return c.SendAll(ctx, messages)
}

// SendMulticastDryRun sends the given multicast message to all the specified FCM registration
// tokens in the dry run (validation only) mode.
//
// This function does not actually deliver any messages to target devices. Instead, it performs all
// the SDK-level and backend validations on the messages, and emulates the send operation.
//
// The tokens array in MulticastMessage may contain up to 500 tokens. SendMulticastDryRun uses the
// `SendAllDryRun()` function to send the given message. The responses list obtained from
// the return value corresponds to the order of the input tokens. An error from SendMulticastDryRun
// indicates a total failure -- i.e. none of the messages were sent to FCM for validation. Partial
// failures are indicated by a `BatchResponse` return value.
func (c *fcmClient) SendMulticastDryRun(ctx context.Context, message *MulticastMessage) (*BatchResponse, error) {
	messages, err := toMessages(message)
	if err != nil {
		return nil, err
	}

	return c.SendAllDryRun(ctx, messages)
}

func toMessages(message *MulticastMessage) ([]*Message, error) {
	if message == nil {
		return nil, errors.New("message must not be nil")
	}

	return message.toMessages()
}

func (c *fcmClient) sendBatch(
	ctx context.Context, messages []*Message, dryRun bool) (*BatchResponse, error) {

	if len(messages) == 0 {
		return nil, errors.New("messages must not be nil or empty")
	}

	if len(messages) > maxMessages {
		return nil, fmt.Errorf("messages must not contain more than %d elements", maxMessages)
	}

	request, err := c.newBatchRequest(messages, dryRun)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(ctx, request)
	if err != nil {
		return nil, err
	}

	if resp.Status != http.StatusOK {
		return nil, handleFCMError(resp)
	}

	return newBatchResponse(resp)
}

// part represents a HTTP request that can be sent embedded in a multipart batch request.
//
// See https://cloud.google.com/compute/docs/api/how-tos/batch for details on how GCP APIs support multipart batch
// requests.
type part struct {
	method  string
	url     string
	headers map[string]string
	body    interface{}
}

// multipartEntity represents an HTTP entity that consists of multiple HTTP requests (parts).
type multipartEntity struct {
	parts []*part
}

func (c *fcmClient) newBatchRequest(messages []*Message, dryRun bool) (*internal.Request, error) {
	url := fmt.Sprintf("%s/projects/%s/messages:send", c.fcmEndpoint, c.project)
	headers := map[string]string{
		apiFormatVersionHeader: apiFormatVersion,
		firebaseClientHeader:   c.version,
	}

	var parts []*part
	for idx, m := range messages {
		if err := validateMessage(m); err != nil {
			return nil, fmt.Errorf("invalid message at index %d: %v", idx, err)
		}

		p := &part{
			method: http.MethodPost,
			url:    url,
			body: &fcmRequest{
				Message:      m,
				ValidateOnly: dryRun,
			},
			headers: headers,
		}
		parts = append(parts, p)
	}

	return &internal.Request{
		Method: http.MethodPost,
		URL:    c.batchEndpoint,
		Body:   &multipartEntity{parts: parts},
		Opts: []internal.HTTPOption{
			internal.WithHeader(firebaseClientHeader, c.version),
		},
	}, nil
}

func newBatchResponse(resp *internal.Response) (*BatchResponse, error) {
	_, params, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		return nil, fmt.Errorf("error parsing content-type header: %v", err)
	}

	mr := multipart.NewReader(bytes.NewBuffer(resp.Body), params["boundary"])
	var responses []*SendResponse
	successCount := 0
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}

		sr, err := newSendResponse(part)
		if err != nil {
			return nil, err
		}

		responses = append(responses, sr)
		if sr.Success {
			successCount++
		}
	}

	return &BatchResponse{
		Responses:    responses,
		SuccessCount: successCount,
		FailureCount: len(responses) - successCount,
	}, nil
}

func newSendResponse(part *multipart.Part) (*SendResponse, error) {
	hr, err := http.ReadResponse(bufio.NewReader(part), nil)
	if err != nil {
		return nil, fmt.Errorf("error parsing multipart body: %v", err)
	}

	b, err := ioutil.ReadAll(hr.Body)
	if err != nil {
		return nil, err
	}

	if hr.StatusCode != http.StatusOK {
		resp := &internal.Response{
			Status: hr.StatusCode,
			Header: hr.Header,
			Body:   b,
		}
		return &SendResponse{
			Success: false,
			Error:   handleFCMError(resp),
		}, nil
	}

	var result fcmResponse
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, err
	}

	return &SendResponse{
		Success:   true,
		MessageID: result.Name,
	}, nil
}

func (e *multipartEntity) Mime() string {
	return fmt.Sprintf("multipart/mixed; boundary=%s", multipartBoundary)
}

func (e *multipartEntity) Bytes() ([]byte, error) {
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	writer.SetBoundary(multipartBoundary)
	for idx, part := range e.parts {
		if err := part.writeTo(writer, idx); err != nil {
			return nil, err
		}
	}

	writer.Close()
	return buffer.Bytes(), nil
}

func (p *part) writeTo(writer *multipart.Writer, idx int) error {
	b, err := p.bytes()
	if err != nil {
		return err
	}

	header := make(textproto.MIMEHeader)
	header.Add("Content-Length", fmt.Sprintf("%d", len(b)))
	header.Add("Content-Type", "application/http")
	header.Add("Content-Id", fmt.Sprintf("%d", idx+1))
	header.Add("Content-Transfer-Encoding", "binary")

	part, err := writer.CreatePart(header)
	if err != nil {
		return err
	}

	_, err = part.Write(b)
	return err
}

func (p *part) bytes() ([]byte, error) {
	b, err := json.Marshal(p.body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(p.method, p.url, bytes.NewBuffer(b))
	if err != nil {
		return nil, err
	}

	for key, value := range p.headers {
		req.Header.Set(key, value)
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("User-Agent", "")

	var buffer bytes.Buffer
	if err := req.Write(&buffer); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}
