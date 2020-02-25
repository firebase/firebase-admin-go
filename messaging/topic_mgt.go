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
	"fmt"
	"net/http"
	"strings"

	"firebase.google.com/go/internal"
)

const (
	iidEndpoint    = "https://iid.googleapis.com/iid/v1"
	iidSubscribe   = "batchAdd"
	iidUnsubscribe = "batchRemove"
)

var iidErrorCodes = map[string]struct{ Code, Msg string }{
	"INVALID_ARGUMENT": {
		invalidArgument,
		"request contains an invalid argument; code: " + invalidArgument,
	},
	"NOT_FOUND": {
		registrationTokenNotRegistered,
		"request contains an invalid argument; code: " + registrationTokenNotRegistered,
	},
	"INTERNAL": {
		internalError,
		"server encountered an internal error; code: " + internalError,
	},
	"TOO_MANY_TOPICS": {
		tooManyTopics,
		"client exceeded the number of allowed topics; code: " + tooManyTopics,
	},
}

// TopicManagementResponse is the result produced by topic management operations.
//
// TopicManagementResponse provides an overview of how many input tokens were successfully handled,
// and how many failed. In case of failures, the Errors list provides specific details concerning
// each error.
type TopicManagementResponse struct {
	SuccessCount int
	FailureCount int
	Errors       []*ErrorInfo
}

func newTopicManagementResponse(resp *iidResponse) *TopicManagementResponse {
	tmr := &TopicManagementResponse{}
	for idx, res := range resp.Results {
		if len(res) == 0 {
			tmr.SuccessCount++
		} else {
			tmr.FailureCount++
			code := res["error"].(string)
			info, ok := iidErrorCodes[code]
			var reason string
			if ok {
				reason = info.Msg
			} else {
				reason = unknownError
			}
			tmr.Errors = append(tmr.Errors, &ErrorInfo{
				Index:  idx,
				Reason: reason,
			})
		}
	}
	return tmr
}

type iidClient struct {
	iidEndpoint string
	httpClient  *internal.HTTPClient
}

func newIIDClient(hc *http.Client) *iidClient {
	client := internal.WithDefaultRetryConfig(hc)
	client.CreateErrFn = handleIIDError
	client.SuccessFn = internal.HasSuccessStatus
	client.Opts = []internal.HTTPOption{internal.WithHeader("access_token_auth", "true")}
	return &iidClient{
		iidEndpoint: iidEndpoint,
		httpClient:  client,
	}
}

// SubscribeToTopic subscribes a list of registration tokens to a topic.
//
// The tokens list must not be empty, and have at most 1000 tokens.
func (c *iidClient) SubscribeToTopic(ctx context.Context, tokens []string, topic string) (*TopicManagementResponse, error) {
	req := &iidRequest{
		Topic:  topic,
		Tokens: tokens,
		op:     iidSubscribe,
	}
	return c.makeTopicManagementRequest(ctx, req)
}

// UnsubscribeFromTopic unsubscribes a list of registration tokens from a topic.
//
// The tokens list must not be empty, and have at most 1000 tokens.
func (c *iidClient) UnsubscribeFromTopic(ctx context.Context, tokens []string, topic string) (*TopicManagementResponse, error) {
	req := &iidRequest{
		Topic:  topic,
		Tokens: tokens,
		op:     iidUnsubscribe,
	}
	return c.makeTopicManagementRequest(ctx, req)
}

type iidRequest struct {
	Topic  string   `json:"to"`
	Tokens []string `json:"registration_tokens"`
	op     string
}

type iidResponse struct {
	Results []map[string]interface{} `json:"results"`
}

type iidError struct {
	Error string `json:"error"`
}

func (c *iidClient) makeTopicManagementRequest(ctx context.Context, req *iidRequest) (*TopicManagementResponse, error) {
	if len(req.Tokens) == 0 {
		return nil, fmt.Errorf("no tokens specified")
	}
	if len(req.Tokens) > 1000 {
		return nil, fmt.Errorf("tokens list must not contain more than 1000 items")
	}
	for _, token := range req.Tokens {
		if token == "" {
			return nil, fmt.Errorf("tokens list must not contain empty strings")
		}
	}

	if req.Topic == "" {
		return nil, fmt.Errorf("topic name not specified")
	}
	if !topicNamePattern.MatchString(req.Topic) {
		return nil, fmt.Errorf("invalid topic name: %q", req.Topic)
	}

	if !strings.HasPrefix(req.Topic, "/topics/") {
		req.Topic = "/topics/" + req.Topic
	}

	request := &internal.Request{
		Method: http.MethodPost,
		URL:    fmt.Sprintf("%s:%s", c.iidEndpoint, req.op),
		Body:   internal.NewJSONEntity(req),
	}
	var result iidResponse
	if _, err := c.httpClient.DoAndUnmarshal(ctx, request, &result); err != nil {
		return nil, err
	}

	return newTopicManagementResponse(&result), nil
}

func handleIIDError(resp *internal.Response) error {
	var ie iidError
	json.Unmarshal(resp.Body, &ie) // ignore any json parse errors at this level
	var clientCode, msg string
	info, ok := iidErrorCodes[ie.Error]
	if ok {
		clientCode, msg = info.Code, info.Msg
	} else {
		clientCode = unknownError
		msg = fmt.Sprintf("client encountered an unknown error; response: %s", string(resp.Body))
	}
	return internal.Errorf(clientCode, "http error status: %d; reason: %s", resp.Status, msg)
}
