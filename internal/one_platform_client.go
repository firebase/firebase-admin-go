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

package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// OnePlatformClient provides APIs for interacting with GCP/Firebase OnePlatform APIs.
type OnePlatformClient struct {
	*HTTPClient
	BaseURL    string
	APIVersion string
	ProjectID  string
	Opts       []HTTPOption
	CreateErr  func(*Response) error
}

// Get makes a GET request.
func (c *OnePlatformClient) Get(
	ctx context.Context, path string, v interface{}) (*Response, error) {
	return c.MakeRequest(ctx, http.MethodGet, path, nil, v)
}

// Post makes a POST request.
func (c *OnePlatformClient) Post(
	ctx context.Context, path string, body interface{}, v interface{}) (*Response, error) {
	return c.MakeRequest(ctx, http.MethodPost, path, body, v)
}

// MakeRequest invokes the remote OnePlatform API, handles any errors and unmarshals the response payload
// into the given variable.
func (c *OnePlatformClient) MakeRequest(
	ctx context.Context, method, path string, body interface{}, v interface{}) (*Response, error) {

	req := &Request{
		Method: method,
		URL:    fmt.Sprintf("%s/%s/projects/%s%s", c.BaseURL, c.APIVersion, c.ProjectID, path),
		Opts:   c.Opts,
	}
	if body != nil {
		req.Body = NewJSONEntity(body)
	}

	resp, err := c.Do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("error while calling remote service: %v", err)
	}

	if resp.Status < http.StatusOK || resp.Status >= http.StatusMultipleChoices {
		return nil, c.handleError(resp)
	}

	if v != nil {
		if err := json.Unmarshal(resp.Body, v); err != nil {
			return nil, fmt.Errorf("error while parsing response: %v", err)
		}
	}

	return resp, nil
}

func (c *OnePlatformClient) handleError(resp *Response) error {
	if c.CreateErr != nil {
		if err := c.CreateErr(resp); err != nil {
			return err
		}
	}

	var httpErr struct {
		Error struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		} `json:"error"`
	}
	json.Unmarshal(resp.Body, &httpErr) // ignore any json parse errors at this level
	code := httpErr.Error.Status
	if code == "" {
		code = "UNKNOWN"
	}

	message := httpErr.Error.Message
	if message == "" {
		message = fmt.Sprintf("unexpected http response with status: %d; body: %s", resp.Status, string(resp.Body))
	}

	return Error(code, message)
}
