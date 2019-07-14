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
// Employs batching to send the entire list as a single RPC call. Compared to the `Send()` function,
// this is a significantly more efficient way to send multiple messages. The responses
// list obtained from the return value corresponds to the order of input messages. An error from
// this function indicates a total failure -- i.e. none of the messages in the list could be sent.
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

type part struct {
	method  string
	url     string
	headers map[string]string
	body    interface{}
}

func (p *part) serialize() ([]byte, error) {
	b, err := json.Marshal(p.body)
	if err != nil {
		return nil, err
	}

	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf("%s %s HTTP/1.1\r\n", p.method, p.url))
	buffer.WriteString(fmt.Sprintf("Content-Length: %d\r\n", len(b)))
	buffer.WriteString("Content-Type: application/json; charset=UTF-8\r\n")
	for key, value := range p.headers {
		buffer.WriteString(fmt.Sprintf("%s: %s\r\n", key, value))
	}
	buffer.WriteString("\r\n")
	buffer.Write(b)
	return buffer.Bytes(), nil
}

type multipartEntity struct {
	parts []*part
}

func (e *multipartEntity) Mime() string {
	return fmt.Sprintf("multipart/mixed; boundary=%s", multipartBoundary)
}

func (e *multipartEntity) Bytes() ([]byte, error) {
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	writer.SetBoundary(multipartBoundary)
	for idx, part := range e.parts {
		if err := writePart(idx, part, writer); err != nil {
			return nil, err
		}
	}

	writer.Close()
	return buffer.Bytes(), nil
}

func writePart(idx int, part *part, writer *multipart.Writer) error {
	body, err := part.serialize()
	if err != nil {
		return err
	}

	header := make(textproto.MIMEHeader)
	header.Add("Content-Length", fmt.Sprintf("%d", len(body)))
	header.Add("Content-Type", "application/http")
	header.Add("Content-Id", fmt.Sprintf("%d", idx+1))
	header.Add("Content-Transfer-Encoding", "binary")

	p, err := writer.CreatePart(header)
	if err != nil {
		return err
	}

	_, err = p.Write(body)
	return err
}

func (c *Client) newBatchRequest(messages []*Message, dryRun bool) (*internal.Request, error) {
	url := fmt.Sprintf("%s/projects/%s/messages:send", c.fcmEndpoint, c.project)
	headers := map[string]string{
		"X-GOOG-API-FORMAT-VERSION": "2",
		"X-FIREBASE-CLIENT":         c.version,
	}

	var parts []*part
	for idx, m := range messages {
		if err := validateMessage(m); err != nil {
			return nil, fmt.Errorf("error validating message at index %d: %v", idx, err)
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
			internal.WithHeader("X-FIREBASE-CLIENT", c.version),
		},
	}, nil
}

func newBatchResponse(resp *internal.Response) (*BatchResponse, error) {
	_, params, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
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

		sr, err := handlePart(part)
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

func handlePart(part *multipart.Part) (*SendResponse, error) {
	b, err := ioutil.ReadAll(part)
	if err != nil {
		return nil, err
	}

	subResp, err := http.ReadResponse(bufio.NewReader(bytes.NewBuffer(b)), nil)
	if err != nil {
		return nil, err
	}

	return newSendResponse(subResp)
}

func newSendResponse(subResp *http.Response) (*SendResponse, error) {
	sb, err := ioutil.ReadAll(subResp.Body)
	if err != nil {
		return nil, err
	}

	if subResp.StatusCode != http.StatusOK {
		resp := &internal.Response{
			Status: subResp.StatusCode,
			Header: subResp.Header,
			Body:   sb,
		}
		return &SendResponse{
			Success: false,
			Error:   handleFCMError(resp),
		}, nil
	}

	var result fcmResponse
	if err := json.Unmarshal(sb, &result); err != nil {
		return nil, err
	}

	return &SendResponse{
		Success:   true,
		MessageID: result.Name,
	}, nil
}
