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
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"

	"firebase.google.com/go/internal"
)

// SendResponse represents the status of an individual message that was sent as part of a batch request.
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
	if len(messages) == 0 {
		return nil, errors.New("messages must not be nil or empty")
	}
	if len(messages) > 100 {
		return nil, errors.New("messages must not contain more than 100 elements")
	}

	url := fmt.Sprintf("%s/projects/%s/messages:send", c.fcmEndpoint, c.project)
	headers := map[string]string{
		"X-GOOG-API-FORMAT-VERSION": "2",
		"X-FIREBASE-CLIENT":         c.version,
	}

	var reqs []*subRequest
	for idx, m := range messages {
		if err := validateMessage(m); err != nil {
			return nil, fmt.Errorf("error validating message at index %d: %v", idx, err)
		}

		r := &subRequest{
			url: url,
			body: &fcmRequest{
				Message: m,
			},
			headers: headers,
		}
		reqs = append(reqs, r)
	}

	request := &internal.Request{
		Method: http.MethodPost,
		URL:    "https://fcm.googleapis.com/batch",
		Body:   &multipartEntity{reqs},
		Opts: []internal.HTTPOption{
			internal.WithHeader("X-FIREBASE-CLIENT", c.version),
		},
	}

	resp, err := c.client.Do(ctx, request)
	if err != nil {
		return nil, err
	}

	if resp.Status != http.StatusOK {
		return nil, c.handleFCMError(resp)
	}

	_, params, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	mr := multipart.NewReader(bytes.NewBuffer(resp.Body), params["boundary"])
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}

		b, err := ioutil.ReadAll(part)
		if err != nil {
			return nil, err
		}
		subResp, err := http.ReadResponse(bufio.NewReader(bytes.NewBuffer(b)), nil)
		if err != nil {
			return nil, err
		}

		sb, _ := ioutil.ReadAll(subResp.Body)
		if subResp.StatusCode == http.StatusOK {
			var result fcmResponse
			json.Unmarshal(sb, &result)
			sr := &SendResponse{
				Success:   true,
				MessageID: result.Name,
			}
			log.Println("SUCCESS", sr)
		} else {

			sr := &SendResponse{
				Success: false,
				Error: c.handleFCMError(&internal.Response{
					Status: subResp.StatusCode,
					Header: subResp.Header,
					Body:   sb,
				}),
			}
			log.Println(sr)
		}

	}
	return nil, nil
}

type multipartEntity struct {
	reqs []*subRequest
}

func (e *multipartEntity) Bytes() ([]byte, error) {
	return multipartPayload(e.reqs)
}

func (e *multipartEntity) Mime() string {
	return "multipart/mixed; boundary=__END_OF_PART__"
}

type subRequest struct {
	url     string
	body    interface{}
	headers map[string]string
}

func (req *subRequest) serialize() ([]byte, error) {
	b, err := json.Marshal(req.body)
	if err != nil {
		return nil, err
	}

	payload := fmt.Sprintf("POST %s HTTP/1.1\r\n", req.url)
	payload += fmt.Sprintf("Content-Length: %d\r\n", len(b))
	payload += "Content-Type: application/json; charset=UTF-8\r\n"
	for key, value := range req.headers {
		payload += fmt.Sprintf("%s: %s\r\n", key, value)
	}
	payload += "\r\n"
	payload += string(b)
	return []byte(payload), nil
}

func multipartPayload(requests []*subRequest) ([]byte, error) {
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	writer.SetBoundary("__END_OF_PART__")
	for idx, req := range requests {
		body, err := req.serialize()
		if err != nil {
			return nil, err
		}

		header := make(textproto.MIMEHeader)
		header.Add("Content-Length", fmt.Sprintf("%d", len(body)))
		header.Add("Content-Type", "application/http")
		header.Add("Content-Id", fmt.Sprintf("%d", idx+1))
		header.Add("Content-Transfer-Encoding", "binary")
		part, err := writer.CreatePart(header)
		if err != nil {
			return nil, err
		}
		if _, err := part.Write(body); err != nil {
			return nil, err
		}
	}
	writer.Close()
	return body.Bytes(), nil
}
