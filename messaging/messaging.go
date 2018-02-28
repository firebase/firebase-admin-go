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
// device subscriptions with Firebase Cloud Messaging (FCM).
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

const (
	messagingEndpoint = "https://fcm.googleapis.com/v1"
	iidEndpoint       = "https://iid.googleapis.com"
	iidSubscribe      = "iid/v1:batchAdd"
	iidUnsubscribe    = "iid/v1:batchRemove"
)

var (
	topicNamePattern = regexp.MustCompile("^(/topics/)?(private/)?[a-zA-Z0-9-_.~%]+$")

	fcmErrorCodes = map[string]string{
		// FCM v1 canonical error codes
		"NOT_FOUND":          "app instance has been unregistered; code: registration-token-not-registered",
		"PERMISSION_DENIED":  "sender id does not match regisration token; code: mismatched-credential",
		"RESOURCE_EXHAUSTED": "messaging service quota exceeded; code: message-rate-exceeded",
		"UNAUTHENTICATED":    "apns certificate or auth key was invalid; code: invalid-apns-credentials",

		// FCM v1 new error codes
		"APNS_AUTH_ERROR":    "apns certificate or auth key was invalid; code: invalid-apns-credentials",
		"INTERNAL":           "back servers encountered an unknown internl error; code: internal-error",
		"INVALID_ARGUMENT":   "request contains an invalid argument; code: invalid-argument",
		"SENDER_ID_MISMATCH": "sender id does not match regisration token; code: mismatched-credential",
		"QUOTA_EXCEEDED":     "messaging service quota exceeded; code: message-rate-exceeded",
		"UNAVAILABLE":        "backend servers are temporarily unavailable; code: server-unavailable",
		"UNREGISTERED":       "app instance has been unregistered; code: registration-token-not-registered",
	}

	iidErrorCodes = map[string]string{
		"INVALID_ARGUMENT": "request contains an invalid argument; code: invalid-argument",
		"NOT_FOUND":        "request contains an invalid argument; code: registration-token-not-registered",
		"INTERNAL":         "server encountered an internal error; code: internal-error",
		"TOO_MANY_TOPICS":  "client exceeded the number of allowed topics; code: too-many-topics",
	}
)

// Client is the interface for the Firebase Cloud Messaging (FCM) service.
type Client struct {
	fcmEndpoint string // to enable testing against arbitrary endpoints
	iidEndpoint string // to enable testing against arbitrary endpoints
	client      *internal.HTTPClient
	project     string
	version     string
}

// Message to be sent via Firebase Cloud Messaging.
//
// Message contains payload data, recipient information and platform-specific configuration
// options. A Message must specify exactly one of Token, Topic or Condition fields. Apart from
// that a Message may specify any combination of Data, Notification, Android, Webpush and APNS
// fields. See https://firebase.google.com/docs/reference/fcm/rest/v1/projects.messages for more
// details on how the backend FCM servers handle different message parameters.
type Message struct {
	Data         map[string]string `json:"data,omitempty"`
	Notification *Notification     `json:"notification,omitempty"`
	Android      *AndroidConfig    `json:"android,omitempty"`
	Webpush      *WebpushConfig    `json:"webpush,omitempty"`
	APNS         *APNSConfig       `json:"apns,omitempty"`
	Token        string            `json:"token,omitempty"`
	Topic        string            `json:"-"`
	Condition    string            `json:"condition,omitempty"`
}

// MarshalJSON marshals a Message into JSON (for internal use only).
func (m *Message) MarshalJSON() ([]byte, error) {
	// Create a new type to prevent infinite recursion.
	type messageInternal Message
	s := &struct {
		BareTopic string `json:"topic,omitempty"`
		*messageInternal
	}{
		BareTopic:       strings.TrimPrefix(m.Topic, "/topics/"),
		messageInternal: (*messageInternal)(m),
	}
	return json.Marshal(s)
}

// Notification is the basic notification template to use across all platforms.
type Notification struct {
	Title string `json:"title,omitempty"`
	Body  string `json:"body,omitempty"`
}

// AndroidConfig contains messaging options specific to the Android platform.
type AndroidConfig struct {
	CollapseKey           string               `json:"collapse_key,omitempty"`
	Priority              string               `json:"priority,omitempty"` // one of "normal" or "high"
	TTL                   *time.Duration       `json:"-"`
	RestrictedPackageName string               `json:"restricted_package_name,omitempty"`
	Data                  map[string]string    `json:"data,omitempty"` // if specified, overrides the Data field on Message type
	Notification          *AndroidNotification `json:"notification,omitempty"`
}

// MarshalJSON marshals an AndroidConfig into JSON (for internal use only).
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

// AndroidNotification is a notification to send to Android devices.
type AndroidNotification struct {
	Title        string   `json:"title,omitempty"` // if specified, overrides the Title field of the Notification type
	Body         string   `json:"body,omitempty"`  // if specified, overrides the Body field of the Notification type
	Icon         string   `json:"icon,omitempty"`
	Color        string   `json:"color,omitempty"` // notification color in #RRGGBB format
	Sound        string   `json:"sound,omitempty"`
	Tag          string   `json:"tag,omitempty"`
	ClickAction  string   `json:"click_action,omitempty"`
	BodyLocKey   string   `json:"body_loc_key,omitempty"`
	BodyLocArgs  []string `json:"body_loc_args,omitempty"`
	TitleLocKey  string   `json:"title_loc_key,omitempty"`
	TitleLocArgs []string `json:"title_loc_args,omitempty"`
}

// WebpushConfig contains messaging options specific to the WebPush protocol.
//
// See https://tools.ietf.org/html/rfc8030#section-5 for additional details, and supported
// headers.
type WebpushConfig struct {
	Headers      map[string]string    `json:"headers,omitempty"`
	Data         map[string]string    `json:"data,omitempty"`
	Notification *WebpushNotification `json:"notification,omitempty"`
}

// WebpushNotification is a notification to send via WebPush protocol.
type WebpushNotification struct {
	Title string `json:"title,omitempty"` // if specified, overrides the Title field of the Notification type
	Body  string `json:"body,omitempty"`  // if specified, overrides the Body field of the Notification type
	Icon  string `json:"icon,omitempty"`
}

// APNSConfig contains messaging options specific to the Apple Push Notification Service (APNS).
//
// See https://developer.apple.com/library/content/documentation/NetworkingInternet/Conceptual/RemoteNotificationsPG/CommunicatingwithAPNs.html
// for more details on supported headers and payload keys.
type APNSConfig struct {
	Headers map[string]string `json:"headers,omitempty"`
	Payload *APNSPayload      `json:"payload,omitempty"`
}

// APNSPayload is the payload that can be included in an APNS message.
//
// The payload mainly consists of the aps dictionary. Additionally it may contain arbitrary
// key-values pairs as custom data fields.
//
// See https://developer.apple.com/library/content/documentation/NetworkingInternet/Conceptual/RemoteNotificationsPG/PayloadKeyReference.html
// for a full list of supported payload fields.
type APNSPayload struct {
	Aps        *Aps
	CustomData map[string]interface{}
}

// MarshalJSON marshals an APNSPayload into JSON (for internal use only).
func (p *APNSPayload) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{"aps": p.Aps}
	for k, v := range p.CustomData {
		m[k] = v
	}
	return json.Marshal(m)
}

// Aps represents the aps dictionary that may be included in an APNSPayload.
//
// Alert may be specified as a string (via the AlertString field), or as a struct (via the Alert
// field).
type Aps struct {
	AlertString      string    `json:"-"`
	Alert            *ApsAlert `json:"-"`
	Badge            *int      `json:"badge,omitempty"`
	Sound            string    `json:"sound,omitempty"`
	ContentAvailable bool      `json:"-"`
	Category         string    `json:"category,omitempty"`
	ThreadID         string    `json:"thread-id,omitempty"`
}

// MarshalJSON marshals an Aps into JSON (for internal use only).
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
	} else if a.AlertString != "" {
		s.Alert = a.AlertString
	}
	if a.ContentAvailable {
		one := 1
		s.ContentAvailable = &one
	}
	return json.Marshal(s)
}

// ApsAlert is the alert payload that can be included in an Aps.
//
// See https://developer.apple.com/library/content/documentation/NetworkingInternet/Conceptual/RemoteNotificationsPG/PayloadKeyReference.html
// for supported fields.
type ApsAlert struct {
	Title        string   `json:"title,omitempty"` // if specified, overrides the Title field of the Notification type
	Body         string   `json:"body,omitempty"`  // if specified, overrides the Body field of the Notification type
	LocKey       string   `json:"loc-key,omitempty"`
	LocArgs      []string `json:"loc-args,omitempty"`
	TitleLocKey  string   `json:"title-loc-key,omitempty"`
	TitleLocArgs []string `json:"title-loc-args,omitempty"`
	ActionLocKey string   `json:"action-loc-key,omitempty"`
	LaunchImage  string   `json:"launch-image,omitempty"`
}

// ErrorInfo is a topic management error.
type ErrorInfo struct {
	Index  int
	Reason string
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
			reason := iidErrorCodes[code]
			if reason == "" {
				reason = "unknown-error"
			}
			tmr.Errors = append(tmr.Errors, &ErrorInfo{
				Index:  idx,
				Reason: reason,
			})
		}
	}
	return tmr
}

// NewClient creates a new instance of the Firebase Cloud Messaging Client.
//
// This function can only be invoked from within the SDK. Client applications should access the
// the messaging service through firebase.App.
func NewClient(ctx context.Context, c *internal.MessagingConfig) (*Client, error) {
	if c.ProjectID == "" {
		return nil, errors.New("project ID is required to access Firebase Cloud Messaging client")
	}

	hc, _, err := transport.NewHTTPClient(ctx, c.Opts...)
	if err != nil {
		return nil, err
	}

	return &Client{
		fcmEndpoint: messagingEndpoint,
		iidEndpoint: iidEndpoint,
		client:      &internal.HTTPClient{Client: hc},
		project:     c.ProjectID,
		version:     "Go/Admin/" + c.Version,
	}, nil
}

// Send sends a Message to Firebase Cloud Messaging.
//
// The Message must specify exactly one of Token, Topic and Condition fields. FCM will
// customize the message for each target platform based on the arguments specified in the
// Message.
func (c *Client) Send(ctx context.Context, message *Message) (string, error) {
	payload := &fcmRequest{
		Message: message,
	}
	return c.makeSendRequest(ctx, payload)
}

// SendDryRun sends a Message to Firebase Cloud Messaging in the dry run (validation only) mode.
//
// This function does not actually deliver the message to target devices. Instead, it performs all
// the SDK-level and backend validations on the message, and emulates the send operation.
func (c *Client) SendDryRun(ctx context.Context, message *Message) (string, error) {
	payload := &fcmRequest{
		ValidateOnly: true,
		Message:      message,
	}
	return c.makeSendRequest(ctx, payload)
}

// SubscribeToTopic subscribes a list of registration tokens to a topic.
//
// The tokens list must not be empty, and have at most 1000 tokens.
func (c *Client) SubscribeToTopic(ctx context.Context, tokens []string, topic string) (*TopicManagementResponse, error) {
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
func (c *Client) UnsubscribeFromTopic(ctx context.Context, tokens []string, topic string) (*TopicManagementResponse, error) {
	req := &iidRequest{
		Topic:  topic,
		Tokens: tokens,
		op:     iidSubscribe,
	}
	return c.makeTopicManagementRequest(ctx, req)
}

type fcmRequest struct {
	ValidateOnly bool     `json:"validate_only,omitempty"`
	Message      *Message `json:"message,omitempty"`
}

type fcmResponse struct {
	Name string `json:"name"`
}

type fcmError struct {
	Error struct {
		Status string `json:"status"`
	} `json:"error"`
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

func (c *Client) makeSendRequest(ctx context.Context, req *fcmRequest) (string, error) {
	if err := validateMessage(req.Message); err != nil {
		return "", err
	}

	request := &internal.Request{
		Method: http.MethodPost,
		URL:    fmt.Sprintf("%s/projects/%s/messages:send", c.fcmEndpoint, c.project),
		Body:   internal.NewJSONEntity(req),
	}
	resp, err := c.client.Do(ctx, request)
	if err != nil {
		return "", err
	}

	if resp.Status == http.StatusOK {
		var result fcmResponse
		err := json.Unmarshal(resp.Body, &result)
		return result.Name, err
	}

	var fe fcmError
	json.Unmarshal(resp.Body, &fe) // ignore any json parse errors at this level
	msg := fcmErrorCodes[fe.Error.Status]
	if msg == "" {
		msg = fmt.Sprintf("server responded with an unknown error; response: %s", string(resp.Body))
	}
	return "", fmt.Errorf("http error status: %d; reason: %s", resp.Status, msg)
}

func (c *Client) makeTopicManagementRequest(ctx context.Context, req *iidRequest) (*TopicManagementResponse, error) {
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
		URL:    fmt.Sprintf("%s/%s", c.iidEndpoint, req.op),
		Body:   internal.NewJSONEntity(req),
		Opts:   []internal.HTTPOption{internal.WithHeader("access_token_auth", "true")},
	}
	resp, err := c.client.Do(ctx, request)
	if err != nil {
		return nil, err
	}

	if resp.Status == http.StatusOK {
		var result iidResponse
		if err := json.Unmarshal(resp.Body, &result); err != nil {
			return nil, err
		}
		return newTopicManagementResponse(&result), nil
	}

	var ie iidError
	json.Unmarshal(resp.Body, &ie) // ignore any json parse errors at this level
	msg := iidErrorCodes[ie.Error]
	if msg == "" {
		msg = fmt.Sprintf("client encountered an unknown error; response: %s", string(resp.Body))
	}
	return nil, fmt.Errorf("http error status: %d; reason: %s", resp.Status, msg)
}
