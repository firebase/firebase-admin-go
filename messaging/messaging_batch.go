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

const multipartBoundary = "__END_OF_PART__"

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

// SendAll sends all the messages in the given array via Firebase Cloud Messaging.
//
// The messages array may contain up to 100 messages. SendAll employs batching to send the entire
// array of mssages as a single RPC call. Compared to the `Send()` function,
// this is a significantly more efficient way to send multiple messages. The responses
// list obtained from the return value corresponds to the order of input messages. An error from
// SendAll indicates a total failure -- i.e. none of the messages in the array could be sent.
// Partial failures are indicated by a `BatchResponse` return value.
func (c *Client) SendAll(ctx context.Context, messages []*Message) (*BatchResponse, error) {
	return c.sendBatch(ctx, messages, false)
}

func (c *Client) sendBatch(
	ctx context.Context, messages []*Message, dryRun bool) (*BatchResponse, error) {

	if len(messages) == 0 {
		return nil, errors.New("messages must not be nil or empty")
	}

	if len(messages) > 100 {
		return nil, errors.New("messages must not contain more than 100 elements")
	}

	request, err := c.newBatchRequest(messages, dryRun)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(ctx, request)
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

func (c *Client) newBatchRequest(messages []*Message, dryRun bool) (*internal.Request, error) {
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
