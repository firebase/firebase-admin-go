// Copyright 2017 Google Inc. All Rights Reserved.
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

// Package messaging contains functions for sending messages and managing
// device subscriptions with Firebase Cloud Messaging.
package messaging

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"firebase.google.com/go/internal"
	"google.golang.org/api/transport"
)

const messagingEndpoint = "https://fcm.googleapis.com/v1"

var errorCodes = map[int]string{
	400: "malformed argument",
	401: "request not authorized",
	403: "project does not match or the client does not have sufficient privileges",
	404: "failed to find the ...",
	409: "already deleted",
	429: "request throttled out by the backend server",
	500: "internal server error",
	503: "backend servers are over capacity",
}

// Client is the interface for the Firebase Messaging service.
type Client struct {
	// To enable testing against arbitrary endpoints.
	endpoint string
	client   *internal.HTTPClient
	project  string
	version  string
}

// RequestMessage is the request body message to send by Firebase Cloud Messaging Service.
// See https://firebase.google.com/docs/reference/fcm/rest/v1/projects.messages/send
type RequestMessage struct {
	ValidateOnly bool    `json:"validate_only"`
	Message      Message `json:"message"`
}

// ResponseMessage is the identifier of the message sent.
// See https://firebase.google.com/docs/reference/fcm/rest/v1/projects.messages
type ResponseMessage struct {
	Name string `json:"name"`
}

// Message is the message to send by Firebase Cloud Messaging Service.
// See https://firebase.google.com/docs/reference/fcm/rest/v1/projects.messages#Message
type Message struct {
	Name         string                 `json:"name"`
	Data         map[string]interface{} `json:"data"`
	Notification Notification           `json:"notification,omitempty"`
	Android      AndroidConfig          `json:"android,omitempty"`
	Webpush      WebpushConfig          `json:"webpush,omitempty"`
	Apns         ApnsConfig             `json:"apns,omitempty"`
	Token        string                 `json:"token,omitempty"`
	Topic        string                 `json:"topic,omitempty"`
	Condition    string                 `json:"condition,omitempty"`
}

// Notification is the Basic notification template to use across all platforms.
// See https://firebase.google.com/docs/reference/fcm/rest/v1/projects.messages#Notification
type Notification struct {
	Title string `json:"title,omitempty"`
	Body  string `json:"body,omitempty"`
}

// AndroidConfig is Android specific options for messages.
// See https://firebase.google.com/docs/reference/fcm/rest/v1/projects.messages#AndroidConfig
type AndroidConfig struct {
	CollapseKey           string                 `json:"collapse_key,omitempty"`
	Priority              string                 `json:"priority,omitempty"`
	TTL                   string                 `json:"ttl,omitempty"`
	RestrictedPackageName string                 `json:"restricted_package_name,omitempty"`
	Data                  map[string]interface{} `json:"data,omitempty"`
	Notification          AndroidNotification    `json:"notification,omitempty"`
}

// AndroidNotification is notification to send to android devices.
// See https://firebase.google.com/docs/reference/fcm/rest/v1/projects.messages#AndroidNotification
type AndroidNotification struct {
	Title        string   `json:"title,omitempty"`
	Body         string   `json:"body,omitempty"`
	Icon         string   `json:"icon,omitempty"`
	Color        string   `json:"color,omitempty"`
	Sound        string   `json:"sound,omitempty"`
	Tag          string   `json:"tag,omitempty"`
	ClickAction  string   `json:"click_action,omitempty"`
	BodyLocKey   string   `json:"body_loc_key,omitempty"`
	BodyLocArgs  []string `json:"body_loc_args,omitempty"`
	TitleLocKey  string   `json:"title_loc_key,omitempty"`
	TitleLocArgs []string `json:"title_loc_args,omitempty"`
}

// WebpushConfig is Webpush protocol options.
// See https://firebase.google.com/docs/reference/fcm/rest/v1/projects.messages#WebpushConfig
type WebpushConfig struct {
	Headers      map[string]interface{} `json:"headers,omitempty"`
	Data         map[string]interface{} `json:"data,omitempty"`
	Notification WebpushNotification    `json:"notification,omitempty"`
}

// WebpushNotification is Web notification to send via webpush protocol.
// See https://firebase.google.com/docs/reference/fcm/rest/v1/projects.messages#WebpushNotification
type WebpushNotification struct {
	Title string `json:"title,omitempty"`
	Body  string `json:"body,omitempty"`
	Icon  string `json:"icon,omitempty"`
}

// ApnsConfig is Apple Push Notification Service specific options.
// See https://firebase.google.com/docs/reference/fcm/rest/v1/projects.messages#ApnsConfig
type ApnsConfig struct {
	Headers map[string]string      `json:"headers,omitempty"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

// NewClient creates a new instance of the Firebase Cloud Messaging Client.
//
// This function can only be invoked from within the SDK. Client applications should access the
// the Messaging service through firebase.App.
func NewClient(ctx context.Context, c *internal.MessagingConfig) (*Client, error) {
	if c.ProjectID == "" {
		return nil, errors.New("project id is required to access firebase cloud messaging client")
	}

	hc, _, err := transport.NewHTTPClient(ctx, c.Opts...)
	if err != nil {
		return nil, err
	}

	return &Client{
		endpoint: messagingEndpoint,
		client:   &internal.HTTPClient{Client: hc},
		project:  c.ProjectID,
		version:  "Go/Admin/" + c.Version,
	}, nil
}

// SendMessage sends a Message to Firebase Cloud Messaging.
//
// Send a message to specified target (a registration token, topic or condition).
// https://firebase.google.com/docs/cloud-messaging/send-message
func (c *Client) SendMessage(ctx context.Context, payload RequestMessage) (msg *ResponseMessage, err error) {
	if err := validateTarget(payload); err != nil {
		return nil, err
	}

	versionHeader := internal.WithHeader("X-Client-Version", c.version)
	request := &internal.Request{
		Method: http.MethodPost,
		URL:    fmt.Sprintf("%s/project/%s/messages:send", c.endpoint, c.project),
		Body:   internal.NewJSONEntity(payload),
		Opts:   []internal.HTTPOption{versionHeader},
	}
	resp, err := c.client.Do(ctx, request)
	if err != nil {
		return nil, err
	}

	if msg, ok := errorCodes[resp.Status]; ok {
		return nil, fmt.Errorf("project id %q: %s", c.project, msg)
	}

	result := &ResponseMessage{}
	err = resp.Unmarshal(http.StatusOK, result)

	return result, err
}

// validators

// TODO add validator : Data messages can have a 4KB maximum payload.
// TODO add validator : topic name reg expression: "[a-zA-Z0-9-_.~%]+".
// TODO add validator : Conditions for topics support two operators per
// expression, and parentheses are supported.

func validateTarget(payload RequestMessage) error {
	if payload.Message.Token == "" && payload.Message.Condition == "" && payload.Message.Topic == "" {
		return fmt.Errorf("target is empty you have to fill one of this fields (Token, Condition, Topic)")
	}
	return nil
}
