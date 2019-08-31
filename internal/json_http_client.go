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

// DoJSON invokes the remote API, handles any errors and unmarshals the response payload
// into the given variable.
func (c *HTTPClient) DoJSON(ctx context.Context, req *Request, v interface{}) (*Response, error) {
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

func (c *HTTPClient) handleError(resp *Response) error {
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
