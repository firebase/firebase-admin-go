// Copyright 2019 Google LLC All Rights Reserved.
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
	"net/url"
	"strings"

	"firebase.google.com/go/v4/internal"
)

const (
	iidEndpoint               = "https://iid.googleapis.com/iid/v1"
	iidSubscribe              = "batchAdd"
	iidUnsubscribe            = "batchRemove"
	maxTopicManagementWorkers = 100
)

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
			reason := res["error"].(string)
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

func newIIDClient(hc *http.Client, conf *internal.MessagingConfig) *iidClient {
	client := internal.WithDefaultRetryConfig(hc)
	client.CreateErrFn = handleIIDError
	client.Opts = []internal.HTTPOption{
		internal.WithHeader("access_token_auth", "true"),
		internal.WithHeader("x-goog-api-client", internal.GetMetricsHeader(conf.Version)),
	}
	return &iidClient{
		iidEndpoint: iidEndpoint,
		httpClient:  client,
	}
}

// SubscribeToTopicLegacy subscribes a list of registration tokens to a topic using the legacy Instance ID API.
//
// Deprecated: Use SubscribeToTopic instead.
func (c *iidClient) SubscribeToTopicLegacy(ctx context.Context, tokens []string, topic string) (*TopicManagementResponse, error) {
	req := &iidRequest{
		Topic:  topic,
		Tokens: tokens,
		op:     iidSubscribe,
	}
	return c.makeTopicManagementRequest(ctx, req)
}

// UnsubscribeFromTopicLegacy unsubscribes a list of registration tokens from a topic using the legacy Instance ID API.
//
// Deprecated: Use UnsubscribeFromTopic instead.
func (c *iidClient) UnsubscribeFromTopicLegacy(ctx context.Context, tokens []string, topic string) (*TopicManagementResponse, error) {
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

type iidErrorResponse struct {
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
	base := internal.NewFirebaseError(resp)
	var ie iidErrorResponse
	json.Unmarshal(resp.Body, &ie) // ignore any json parse errors at this level
	if ie.Error != "" {
		base.Message = fmt.Sprintf("error while calling the iid service: %s", ie.Error)
	}

	return base
}

func validateTopicManagementArgs(tokens []string, topic string) (string, error) {
	if len(tokens) == 0 {
		return "", fmt.Errorf("no tokens specified")
	}
	if len(tokens) > 1000 {
		return "", fmt.Errorf("tokens list must not contain more than 1000 items")
	}
	for _, token := range tokens {
		if token == "" {
			return "", fmt.Errorf("tokens list must not contain empty strings")
		}
	}

	if topic == "" {
		return "", fmt.Errorf("topic name not specified")
	}
	if !topicNamePattern.MatchString(topic) {
		return "", fmt.Errorf("invalid topic name: %q", topic)
	}

	topicName := strings.TrimPrefix(topic, "/topics/")
	return topicName, nil
}

type topicJob struct {
	token string
	index int
}

type topicResult struct {
	index   int
	success bool
	reason  string
}

// SubscribeToTopic subscribes a list of registration tokens to a topic via the FCM v1 API.
//
// The tokens list must not be empty, and have at most 1000 tokens.
func (c *fcmClient) SubscribeToTopic(ctx context.Context, tokens []string, topic string) (*TopicManagementResponse, error) {
	return c.makeTopicManagementRequestV1(ctx, tokens, topic, true)
}

// UnsubscribeFromTopic unsubscribes a list of registration tokens from a topic via the FCM v1 API.
//
// The tokens list must not be empty, and have at most 1000 tokens.
func (c *fcmClient) UnsubscribeFromTopic(ctx context.Context, tokens []string, topic string) (*TopicManagementResponse, error) {
	return c.makeTopicManagementRequestV1(ctx, tokens, topic, false)
}

func (c *fcmClient) makeTopicManagementRequestV1(ctx context.Context, tokens []string, topic string, isSubscribe bool) (*TopicManagementResponse, error) {
	topicName, err := validateTopicManagementArgs(tokens, topic)
	if err != nil {
		return nil, err
	}

	numWorkers := len(tokens)
	if numWorkers > maxTopicManagementWorkers {
		numWorkers = maxTopicManagementWorkers
	}

	jobs := make(chan topicJob, len(tokens))
	results := make(chan topicResult, len(tokens))

	for w := 0; w < numWorkers; w++ {
		go func() {
			for j := range jobs {
				success, reason := c.makeTopicManagementSingleRequest(ctx, j.token, topicName, isSubscribe)
				results <- topicResult{
					index:   j.index,
					success: success,
					reason:  reason,
				}
			}
		}()
	}

	for idx, token := range tokens {
		jobs <- topicJob{token: token, index: idx}
	}
	close(jobs)

	resps := make([]topicResult, len(tokens))
	for i := 0; i < len(tokens); i++ {
		res := <-results
		resps[res.index] = res
	}

	tmr := &TopicManagementResponse{}
	for _, res := range resps {
		if res.success {
			tmr.SuccessCount++
		} else {
			tmr.FailureCount++
			tmr.Errors = append(tmr.Errors, &ErrorInfo{
				Index:  res.index,
				Reason: res.reason,
			})
		}
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return tmr, nil
}

func (c *fcmClient) makeTopicManagementSingleRequest(ctx context.Context, token, topicName string, isSubscribe bool) (bool, string) {
	encodedToken := url.PathEscape(token)
	var request *internal.Request

	if isSubscribe {
		request = &internal.Request{
			Method: http.MethodPost,
			URL:    fmt.Sprintf("%s/projects/%s/registrations/%s/topicSubscriptions?topic_name=%s", c.fcmEndpoint, c.project, encodedToken, url.QueryEscape(topicName)),
			Body:   internal.NewJSONEntity(map[string]interface{}{}),
			SuccessFn: func(resp *internal.Response) bool {
				return internal.HasSuccessStatus(resp) || resp.Status == http.StatusConflict
			},
		}
	} else {
		request = &internal.Request{
			Method: http.MethodDelete,
			URL:    fmt.Sprintf("%s/projects/%s/registrations/%s/topicSubscriptions/%s?allow_missing=true", c.fcmEndpoint, c.project, encodedToken, url.PathEscape(topicName)),
		}
	}

	_, err := c.httpClient.Do(ctx, request)
	if err == nil {
		return true, ""
	}

	if fe, ok := err.(*internal.FirebaseError); ok {
		if code, ok := fe.Ext["messagingErrorCode"].(string); ok && code != "" {
			return false, code
		}
		if fe.ErrorCode != "" && fe.ErrorCode != internal.Unknown {
			return false, string(fe.ErrorCode)
		}
	}

	return false, "UNKNOWN_ERROR"
}
