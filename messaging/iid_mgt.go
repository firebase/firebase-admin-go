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
	"fmt"
	"net/http"

	"firebase.google.com/go/v4/internal"
)

const iidImport = "batchImport"

// RegistrationToken is the result produced by Instance ID service's batchImport method.
type RegistrationToken struct {
	ApnsToken         string `json:"apns_token"`
	Status            string `json:"status"`
	RegistrationToken string `json:"registration_token,omitempty"`
}

type iidBatchImportRequest struct {
	Application string   `json:"application"`
	Sandbox     bool     `json:"sandbox"`
	ApnsTokens  []string `json:"apns_tokens"`
}

type iidRegistrationTokens struct {
	Results []RegistrationToken `json:"results"`
}

// GetRegistrationFromAPNs Create registration tokens for APNs tokens.
//
// Using the Instance ID service's batchImport method, you can bulk import existing iOS APNs tokens to
// Firebase Cloud Messaging, mapping them to valid registration tokens.
//
// The response contains an array of Instance ID registration tokens ready to be used for
// sending FCM messages to the corresponding APNs device token.
//
// https://developers.google.com/instance-id/reference/server#create_registration_tokens_for_apns_tokens
func (c *iidClient) GetRegistrationFromAPNs(
	ctx context.Context,
	application string,
	tokens []string,
) ([]RegistrationToken, error) {
	return c.getRegistrationFromAPNs(ctx, application, tokens, false)
}

// GetRegistrationFromAPNsDryRun Create registration tokens for APNs tokens in the dry run (sandbox) mode.
func (c *iidClient) GetRegistrationFromAPNsDryRun(
	ctx context.Context,
	application string,
	tokens []string,
) ([]RegistrationToken, error) {
	return c.getRegistrationFromAPNs(ctx, application, tokens, true)
}

func (c *iidClient) getRegistrationFromAPNs(
	ctx context.Context,
	application string,
	tokens []string,
	sandbox bool,
) ([]RegistrationToken, error) {
	if application == "" {
		return nil, fmt.Errorf("application id not specified")
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("no APNs tokens specified")
	}
	if len(tokens) > 100 {
		return nil, fmt.Errorf("too many APNs tokens specified")
	}
	for i := range tokens {
		if len(tokens[i]) == 0 {
			return nil, fmt.Errorf("tokens list must not contain empty strings")
		}
	}

	request := &internal.Request{
		Method: http.MethodPost,
		URL:    fmt.Sprintf("%s:%s", c.iidEndpoint, iidImport),
		Body: internal.NewJSONEntity(&iidBatchImportRequest{
			Application: application,
			Sandbox:     sandbox,
			ApnsTokens:  tokens,
		}),
	}

	var result iidRegistrationTokens
	if _, err := c.httpClient.DoAndUnmarshal(ctx, request, &result); err != nil {
		return nil, err
	}
	return result.Results, nil
}

// Topics information about relations associated with the token.
type Topics map[string]struct {
	AddDate string `json:"addDate"`
}

// TokenDetails information about app instances.
// Object containing:
//
//	application - package name associated with the token.
//	authorizedEntity - projectId authorized to send to the token.
//	applicationVersion - version of the application.
//	platform - returns ANDROID, IOS, or CHROME to indicate the device platform to which the token belongs.
//	rel - relations associated with the token. For example, a list of topic subscriptions.
type TokenDetails struct {
	ApplicationVersion string `json:"applicationVersion"`
	Application        string `json:"application"`
	AuthorizedEntity   string `json:"authorizedEntity"`
	Rel                struct {
		Topics Topics `json:"topics"`
	} `json:"rel"`
	Platform string `json:"platform"`
}

// GetSubscriptions Get information about relations associated with the token.
func (c *iidClient) GetSubscriptions(ctx context.Context, token string) (Topics, error) {
	res, err := c.GetTokenDetails(ctx, token)
	if err != nil {
		return nil, err
	}
	return res.Rel.Topics, nil
}

// GetTokenDetails Get information about app instances
// On success the call returns object TokenDetails
//
// https://developers.google.com/instance-id/reference/server#get_information_about_app_instances
func (c *iidClient) GetTokenDetails(ctx context.Context, token string) (*TokenDetails, error) {
	if token == "" {
		return nil, fmt.Errorf("token not specified")
	}

	request := &internal.Request{
		Method: http.MethodGet,
		URL:    fmt.Sprintf("%s:/info/%s?details=true", c.iidEndpoint, token),
	}
	var result TokenDetails
	if _, err := c.httpClient.DoAndUnmarshal(ctx, request, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
