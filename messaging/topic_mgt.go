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
	"time"

	"firebase.google.com/go/internal"
)

const (
	iidBaseEndpoint = "https://iid.googleapis.com/iid"
	iidEndpoint     = iidBaseEndpoint + "/v1"
	iidInfoEndpoint = iidBaseEndpoint + "/info"
	iidSubscribe    = "batchAdd"
	iidUnsubscribe  = "batchRemove"
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

// TopicSubscriptionInfoResponse is the result produced by querying the app instance with a token.
//
// TopicSubscriptionInfoResponse contains topic subscription information associated with the input
// token; in particular for each topic the date when that topic was associated with the token is
// provided.
type TopicSubscriptionInfoResponse struct {
	TopicMap map[string]*TopicInfo // TopicMap key is the topic name
}

// TopicInfo is a topic detail information.
type TopicInfo struct {
	Name    string
	AddDate time.Time
}

type iidInfoClient struct {
	iidInfoEndpoint string
	httpClient      *internal.HTTPClient
}

func newIIDInfoClient(hc *http.Client) *iidInfoClient {
	client := internal.WithDefaultRetryConfig(hc)
	client.CreateErrFn = handleIIDInfoError
	client.SuccessFn = internal.HasSuccessStatus
	client.Opts = []internal.HTTPOption{internal.WithHeader("access_token_auth", "true"), internal.WithQueryParam("details", "true")}
	return &iidInfoClient{
		iidInfoEndpoint: iidInfoEndpoint,
		httpClient:      client,
	}
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

// TopicSubscriptionInfo returns a list of topic subscriptions associated with the provided token.
func (c *iidInfoClient) TopicSubscriptionInfo(ctx context.Context, token string) (*TopicSubscriptionInfoResponse, error) {
	if token == "" {
		return nil, fmt.Errorf("no token specified")
	}

	request := &internal.Request{
		Method: http.MethodGet,
		URL:    fmt.Sprintf("%s/%s", c.iidInfoEndpoint, token),
	}
	var result iidInfoResponse
	if _, err := c.httpClient.DoAndUnmarshal(ctx, request, &result); err != nil {
		return nil, err
	}

	tsir := &TopicSubscriptionInfoResponse{}
	tsir.TopicMap = make(map[string]*TopicInfo)
	if result.Rel != nil && result.Rel.Topics != nil {
		for k, v := range *result.Rel.Topics {
			tsir.TopicMap[k] = &TopicInfo{
				Name:    k,
				AddDate: v.AddDate,
			}
		}
	}
	return tsir, nil
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

type iidInfoResponse struct {
	Rel *iidInfoRel `json:"rel,omitempty"`
}

type iidInfoRel struct {
	Topics *map[string]iidTopicAddDateInfo `json:"topics,omitempty"`
}

type iidTopicAddDateInfo struct {
	AddDate time.Time `json:"-"`
}

// UnmarshalJSON unmarshals a JSON string into a iidTopicAddDateInfo (for internal use only)
func (i *iidTopicAddDateInfo) UnmarshalJSON(b []byte) error {
	type iidTopicAddDateInfoInternal iidTopicAddDateInfo
	s := struct {
		AddDateString string `json:"addDate"`
		*iidTopicAddDateInfoInternal
	}{
		iidTopicAddDateInfoInternal: (*iidTopicAddDateInfoInternal)(i),
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	if d, err := time.Parse("2006-01-02", s.AddDateString); err != nil {
		return fmt.Errorf("invalid date: %q", s.AddDateString)
	} else {
		i.AddDate = d
	}
	return nil
}

func handleIIDInfoError(resp *internal.Response) error {
	var ie iidError
	json.Unmarshal(resp.Body, &ie) // ignore any json parse errors at this level
	if resp.Status == http.StatusBadRequest {
		return internal.Errorf(invalidArgument, "request contains an invalid argument; reason: %s", ie.Error)
	} else {
		return internal.Errorf(unknownError, "client encountered an unknown error; response: %s", string(resp.Body))
	}
}
