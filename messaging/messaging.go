// Copyright 2018 Google Inc. All Rights Reserved.
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
	"encoding/json"
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
	http.StatusServiceUnavailable:  "backend server is unavailable",
}

// Client is the interface for the Firebase Messaging service.
type Client struct {
	// To enable testing against arbitrary endpoints.
	endpoint string
	client   *internal.HTTPClient
	project  string
	version  string
}

// requestMessage is the request body message to send by Firebase Cloud Messaging Service.
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
	Data         map[string]string `json:"data,omitempty"`
	Notification *Notification     `json:"notification,omitempty"`
	Android      *AndroidConfig    `json:"android,omitempty"`
	Webpush      *WebpushConfig    `json:"webpush,omitempty"`
	APNS         *APNSConfig       `json:"apns,omitempty"`
	Token        string            `json:"token,omitempty"`
	Topic        string            `json:"topic,omitempty"`
	Condition    string            `json:"condition,omitempty"`
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
	TTL                   *time.Duration       `json:"-"`
	RestrictedPackageName string               `json:"restricted_package_name,omitempty"`
	Data                  map[string]string    `json:"data,omitempty"`
	Notification          *AndroidNotification `json:"notification,omitempty"`
}

func (a *AndroidConfig) MarshalJSON() ([]byte, error) {
	var ttl string
	if a.TTL != nil {
		seconds := int64(*a.TTL / time.Second)
		nanos := int64((*a.TTL - time.Duration(seconds)*time.Second) / time.Nanosecond)
		if nanos > 0 {
			ttl = fmt.Sprintf("%d.%09ds", seconds, nanos)
		} else {
			ttl = fmt.Sprintf("%ds", seconds)
		}
	}

	type androidInternal AndroidConfig
	s := &struct {
		TTL string `json:"ttl,omitempty"`
		*androidInternal
	}{
		TTL:             ttl,
		androidInternal: (*androidInternal)(a),
	}
	return json.Marshal(s)
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
	Payload *APNSPayload      `json:"payload,omitempty"`
}

// APNSPayload is the payload object that can be included in an APNS message.
//
// The payload consists of an aps dictionary, and other custom key-value pairs.
type APNSPayload struct {
	Aps        *Aps
	CustomData map[string]interface{}
}

func (p *APNSPayload) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{"aps": p.Aps}
	for k, v := range p.CustomData {
		m[k] = v
	}
	return json.Marshal(m)
}

type Aps struct {
	AlertString      string    `json:"-"`
	Alert            *ApsAlert `json:"-"`
	Badge            int       `json:"badge,omitempty"`
	Sound            string    `json:"sound,omitempty"`
	ContentAvailable bool      `json:"-"`
	Category         string    `json:"category,omitempty"`
	ThreadID         string    `json:"thread-id,omitempty"`
}

func (a *Aps) MarshalJSON() ([]byte, error) {
	type apsAlias Aps
	s := &struct {
		Alert            interface{} `json:"alert,omitempty"`
		ContentAvailable *int        `json:"content-available,omitempty"`
		*apsAlias
	}{
		apsAlias: (*apsAlias)(a),
	}

	if a.Alert != nil {
		s.Alert = a.Alert
	} else {
		s.Alert = a.AlertString
	}
	if a.ContentAvailable {
		one := 1
		s.ContentAvailable = &one
	}
	return json.Marshal(s)
}

// ApsAlert is the alert payload that can be included in an APNS message.
//
// See https://developer.apple.com/library/content/documentation/NetworkingInternet/Conceptual/RemoteNotificationsPG/PayloadKeyReference.html
// for supported fields.
type ApsAlert struct {
	Title        string   `json:"title,omitempty"`
	Body         string   `json:"body,omitempty"`
	LocKey       string   `json:"loc-key,omitempty"`
	LocArgs      []string `json:"loc-args,omitempty"`
	TitleLocKey  string   `json:"title-loc-key,omitempty"`
	TitleLocArgs []string `json:"title-loc-args,omitempty"`
	ActionLocKey string   `json:"action-loc-key,omitempty"`
	LaunchImage  string   `json:"launch-image,omitempty"`
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
	payload := &requestMessage{
		ValidateOnly: true,
		Message:      message,
	}
	return c.sendRequestMessage(ctx, payload)
}

func (c *Client) sendRequestMessage(ctx context.Context, payload *requestMessage) (string, error) {
	if err := validateMessage(payload.Message); err != nil {
		return "", err
	}

	request := &internal.Request{
		Method: http.MethodPost,
		URL:    fmt.Sprintf("%s/projects/%s/messages:send", c.endpoint, c.project),
		Body:   internal.NewJSONEntity(payload),
	}
	resp, err := c.client.Do(ctx, request)
	if err != nil {
		return "", err
	}

	result := &responseMessage{}
	if err := resp.Unmarshal(http.StatusOK, result); err != nil {
		return "", err
	}
	return result.Name, nil
}

// validateMessage
func validateMessage(message *Message) error {
	if message == nil {
		return fmt.Errorf("message must not be nil")
	}

	target := bool2int(message.Token != "") + bool2int(message.Condition != "") + bool2int(message.Topic != "")
	if target != 1 {
		return fmt.Errorf("exactly one of token, topic or condition must be specified")
	}

	// Validate topic
	if message.Topic != "" {
		if strings.HasPrefix(message.Topic, "/topics/") {
			return fmt.Errorf("topic name must not contain the /topics/ prefix")
		}
		if !regexp.MustCompile("^[a-zA-Z0-9-_.~%]+$").MatchString(message.Topic) {
			return fmt.Errorf("malformed topic name")
		}
	}

	// validate AndroidConfig
	if err := validateAndroidConfig(message.Android); err != nil {
		return err
	}

	// Validate APNSConfig
	return validateAPNSConfig(message.APNS)
}

func validateAndroidConfig(config *AndroidConfig) error {
	if config == nil {
		return nil
	}

	if config.TTL != nil && config.TTL.Seconds() < 0 {
		return fmt.Errorf("ttl duration must not be negative")
	}
	if config.Priority != "" && config.Priority != "normal" && config.Priority != "high" {
		return fmt.Errorf("priority must be 'normal' or 'high'")
	}
	// validate AndroidNotification
	return validateAndroidNotification(config.Notification)
}

func validateAndroidNotification(notification *AndroidNotification) error {
	if notification == nil {
		return nil
	}
	const colorPattern = "^#[0-9a-fA-F]{6}$"
	if notification.Color != "" && !regexp.MustCompile(colorPattern).MatchString(notification.Color) {
		return fmt.Errorf("color must be in the #RRGGBB form")
	}
	if len(notification.TitleLocArgs) > 0 && notification.TitleLocKey == "" {
		return fmt.Errorf("titleLocKey is required when specifying titleLocArgs")
	}
	if len(notification.BodyLocArgs) > 0 && notification.BodyLocKey == "" {
		return fmt.Errorf("bodyLocKey is required when specifying bodyLocArgs")
	}
	return nil
}

func validateAPNSConfig(config *APNSConfig) error {
	if config != nil {
		return validateAPNSPayload(config.Payload)
	}
	return nil
}

func validateAPNSPayload(payload *APNSPayload) error {
	if payload != nil {
		return validateAps(payload.Aps)
	}
	return nil
}

func validateAps(aps *Aps) error {
	if aps != nil {
		if aps.Alert != nil && aps.AlertString != "" {
			return fmt.Errorf("multiple alert specifications")
		}
		return validateApsAlert(aps.Alert)
	}
	return nil
}

func validateApsAlert(alert *ApsAlert) error {
	if alert == nil {
		return nil
	}
	if len(alert.TitleLocArgs) > 0 && alert.TitleLocKey == "" {
		return fmt.Errorf("titleLocKey is required when specifying titleLocArgs")
	}
	if len(alert.LocArgs) > 0 && alert.LocKey == "" {
		return fmt.Errorf("locKey is required when specifying locArgs")
	}
	return nil
}

func bool2int(b bool) int8 {
	if b {
		return 1
	}
	return 0
}
