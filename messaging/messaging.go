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
	"regexp"
	"strings"
	"time"

	"firebase.google.com/go/internal"
	"google.golang.org/api/transport"
)

const messagingEndpoint = "https://fcm.googleapis.com/v1"

var errorCodes = map[int]string{
	http.StatusBadRequest:          "malformed argument",
	http.StatusUnauthorized:        "request not authorized",
	http.StatusForbidden:           "project does not match or the client does not have sufficient privileges",
	http.StatusNotFound:            "failed to find the ...",
	http.StatusConflict:            "already deleted",
	http.StatusTooManyRequests:     "request throttled out by the backend server",
	http.StatusInternalServerError: "internal server error",
	http.StatusServiceUnavailable:  "backend servers are over capacity",
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
type requestMessage struct {
	ValidateOnly bool     `json:"validate_only,omitempty"`
	Message      *Message `json:"message,omitempty"`
}

// responseMessage is the identifier of the message sent.
// See https://firebase.google.com/docs/reference/fcm/rest/v1/projects.messages
type responseMessage struct {
	Name string `json:"name"`
}

// Message is the message to send by Firebase Cloud Messaging Service.
// See https://firebase.google.com/docs/reference/fcm/rest/v1/projects.messages#Message
type Message struct {
	Name         string                 `json:"name,omitempty"`
	Data         map[string]interface{} `json:"data,omitempty"`
	Notification *Notification          `json:"notification,omitempty"`
	Android      *AndroidConfig         `json:"android,omitempty"`
	Webpush      *WebpushConfig         `json:"webpush,omitempty"`
	APNS         *APNSConfig            `json:"apns,omitempty"`
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
	CollapseKey           string               `json:"collapse_key,omitempty"`
	Priority              string               `json:"priority,omitempty"`
	TTL                   string               `json:"ttl,omitempty"`
	RestrictedPackageName string               `json:"restricted_package_name,omitempty"`
	Data                  map[string]string    `json:"data,omitempty"`
	Notification          *AndroidNotification `json:"notification,omitempty"`
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
	Headers      map[string]string    `json:"headers,omitempty"`
	Data         map[string]string    `json:"data,omitempty"`
	Notification *WebpushNotification `json:"notification,omitempty"`
}

// WebpushNotification is Web notification to send via webpush protocol.
// See https://firebase.google.com/docs/reference/fcm/rest/v1/projects.messages#WebpushNotification
type WebpushNotification struct {
	Title string `json:"title,omitempty"`
	Body  string `json:"body,omitempty"`
	Icon  string `json:"icon,omitempty"`
}

// APNSConfig is Apple Push Notification Service specific options.
// See https://firebase.google.com/docs/reference/fcm/rest/v1/projects.messages#apnsconfig
type APNSConfig struct {
	Headers map[string]string `json:"headers,omitempty"`
	Payload map[string]string `json:"payload,omitempty"`
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

// Send sends a Message to Firebase Cloud Messaging.
//
// Send a message to specified target (a registration token, topic or condition).
// https://firebase.google.com/docs/cloud-messaging/send-message
func (c *Client) Send(ctx context.Context, message *Message) (string, error) {
	if err := validateMessage(message); err != nil {
		return "", err
	}
	payload := &requestMessage{
		Message: message,
	}
	return c.sendRequestMessage(ctx, payload)
}

// SendDryRun sends a dryRun Message to Firebase Cloud Messaging.
//
// Send a message to specified target (a registration token, topic or condition).
// https://firebase.google.com/docs/cloud-messaging/send-message
func (c *Client) SendDryRun(ctx context.Context, message *Message) (string, error) {
	if err := validateMessage(message); err != nil {
		return "", err
	}
	payload := &requestMessage{
		ValidateOnly: true,
		Message:      message,
	}
	return c.sendRequestMessage(ctx, payload)
}

func (c *Client) sendRequestMessage(ctx context.Context, payload *requestMessage) (string, error) {
	versionHeader := internal.WithHeader("X-Client-Version", c.version)

	request := &internal.Request{
		Method: http.MethodPost,
		URL:    fmt.Sprintf("%s/projects/%s/messages:send", c.endpoint, c.project),
		Body:   internal.NewJSONEntity(payload),
		Opts:   []internal.HTTPOption{versionHeader},
	}
	resp, err := c.client.Do(ctx, request)
	if err != nil {
		return "", err
	}

	if _, ok := errorCodes[resp.Status]; ok {
		return "", fmt.Errorf("unexpected http status code : %d, reason: %v", resp.Status, string(resp.Body))
	}

	result := &responseMessage{}
	err = resp.Unmarshal(http.StatusOK, result)

	return result.Name, err
}

// validateMessage
func validateMessage(message *Message) error {
	if message == nil {
		return fmt.Errorf("message is empty")
	}

	target := bool2int(message.Token != "") + bool2int(message.Condition != "") + bool2int(message.Topic != "")
	if target != 1 {
		return fmt.Errorf("Exactly one of token, topic or condition must be specified")
	}

	// Validate target
	if message.Topic != "" {
		if strings.HasPrefix(message.Topic, "/topics/") {
			return fmt.Errorf("Topic name must not contain the /topics/ prefix")
		}
		if !regexp.MustCompile("[a-zA-Z0-9-_.~%]+").MatchString(message.Topic) {
			return fmt.Errorf("Malformed topic name")
		}
	}

	// validate AndroidConfig
	if message.Android != nil {
		if err := validateAndroidConfig(message.Android); err != nil {
			return err
		}
	}

	return nil
}

func validateAndroidConfig(config *AndroidConfig) error {
	if config.TTL != "" && !strings.HasSuffix(config.TTL, "s") {
		return fmt.Errorf("ttl must end with 's'")
	}

	if _, err := time.ParseDuration(config.TTL); err != nil {
		return fmt.Errorf("invalid TTL")
	}

	if config.Priority != "" {
		if config.Priority != "normal" && config.Priority != "high" {
			return fmt.Errorf("priority must be 'normal' or 'high'")
		}
	}
	// validate AndroidNotification
	if config.Notification != nil {
		if err := validateAndroidNotification(config.Notification); err != nil {
			return err
		}
	}
	return nil
}

func validateAndroidNotification(notification *AndroidNotification) error {
	if notification.Color != "" {
		if !regexp.MustCompile("^#[0-9a-fA-F]{6}$").MatchString(notification.Color) {
			return fmt.Errorf("color must be in the form #RRGGBB")
		}
	}
	if len(notification.TitleLocArgs) > 0 {
		if notification.TitleLocKey == "" {
			return fmt.Errorf("titleLocKey is required when specifying titleLocArgs")
		}
	}
	if len(notification.BodyLocArgs) > 0 {
		if notification.BodyLocKey == "" {
			return fmt.Errorf("bodyLocKey is required when specifying bodyLocArgs")
		}
	}
	return nil
}

func bool2int(b bool) int8 {
	if b {
		return 1
	}
	return 0
}
