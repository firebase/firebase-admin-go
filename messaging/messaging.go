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
package messaging // import "firebase.google.com/go/messaging"

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"firebase.google.com/go/internal"
	"golang.org/x/net/context"

	"google.golang.org/api/transport"
)

const (
	messagingEndpoint = "https://fcm.googleapis.com/v1"
	iidEndpoint       = "https://iid.googleapis.com"
	iidSubscribe      = "iid/v1:batchAdd"
	iidUnsubscribe    = "iid/v1:batchRemove"

	internalError                  = "internal-error"
	invalidAPNSCredentials         = "invalid-apns-credentials"
	invalidArgument                = "invalid-argument"
	messageRateExceeded            = "message-rate-exceeded"
	mismatchedCredential           = "mismatched-credential"
	registrationTokenNotRegistered = "registration-token-not-registered"
	serverUnavailable              = "server-unavailable"
	tooManyTopics                  = "too-many-topics"
	unknownError                   = "unknown-error"
)

var (
	topicNamePattern = regexp.MustCompile("^(/topics/)?(private/)?[a-zA-Z0-9-_.~%]+$")

	fcmErrorCodes = map[string]struct{ Code, Msg string }{
		// FCM v1 canonical error codes
		"NOT_FOUND": {
			registrationTokenNotRegistered,
			"app instance has been unregistered; code: " + registrationTokenNotRegistered,
		},
		"PERMISSION_DENIED": {
			mismatchedCredential,
			"sender id does not match regisration token; code: " + mismatchedCredential,
		},
		"RESOURCE_EXHAUSTED": {
			messageRateExceeded,
			"messaging service quota exceeded; code: " + messageRateExceeded,
		},
		"UNAUTHENTICATED": {
			invalidAPNSCredentials,
			"apns certificate or auth key was invalid; code: " + invalidAPNSCredentials,
		},

		// FCM v1 new error codes
		"APNS_AUTH_ERROR": {
			invalidAPNSCredentials,
			"apns certificate or auth key was invalid; code: " + invalidAPNSCredentials,
		},
		"INTERNAL": {
			internalError,
			"backend servers encountered an unknown internl error; code: " + internalError,
		},
		"INVALID_ARGUMENT": {
			invalidArgument,
			"request contains an invalid argument; code: " + invalidArgument,
		},
		"SENDER_ID_MISMATCH": {
			mismatchedCredential,
			"sender id does not match regisration token; code: " + mismatchedCredential,
		},
		"QUOTA_EXCEEDED": {
			messageRateExceeded,
			"messaging service quota exceeded; code: " + messageRateExceeded,
		},
		"UNAVAILABLE": {
			serverUnavailable,
			"backend servers are temporarily unavailable; code: " + serverUnavailable,
		},
		"UNREGISTERED": {
			registrationTokenNotRegistered,
			"app instance has been unregistered; code: " + registrationTokenNotRegistered,
		},
	}

	iidErrorCodes = map[string]struct{ Code, Msg string }{
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

// WebpushNotificationAction represents an action that can be performed upon receiving a WebPush notification.
type WebpushNotificationAction struct {
	Action string `json:"action,omitempty"`
	Title  string `json:"title,omitempty"`
	Icon   string `json:"icon,omitempty"`
}

// WebpushNotification is a notification to send via WebPush protocol.
//
// See https://developer.mozilla.org/en-US/docs/Web/API/notification/Notification for additional
// details.
type WebpushNotification struct {
	Actions            []*WebpushNotificationAction
	Title              string      `json:"title,omitempty"` // if specified, overrides the Title field of the Notification type
	Body               string      `json:"body,omitempty"`  // if specified, overrides the Body field of the Notification type
	Icon               string      `json:"icon,omitempty"`
	Badge              string      `json:"badge,omitempty"`
	Direction          string      `json:"dir,omitempty"` // one of 'ltr' or 'rtl'
	Data               interface{} `json:"data,omitempty"`
	Image              string      `json:"image,omitempty"`
	Language           string      `json:"lang,omitempty"`
	Renotify           bool        `json:"renotify,omitempty"`
	RequireInteraction bool        `json:"requireInteraction,omitempty"`
	Silent             bool        `json:"silent,omitempty"`
	Tag                string      `json:"tag,omitempty"`
	TimestampMillis    *int64      `json:"timestamp,omitempty"`
	Vibrate            []int       `json:"vibrate,omitempty"`
	CustomData         map[string]interface{}
}

// standardFields creates a map containing all the fields except the custom data.
func (n *WebpushNotification) standardFields() map[string]interface{} {
	m := make(map[string]interface{})
	addNonEmpty := func(key, value string) {
		if value != "" {
			m[key] = value
		}
	}
	addTrue := func(key string, value bool) {
		if value {
			m[key] = value
		}
	}
	if len(n.Actions) > 0 {
		m["actions"] = n.Actions
	}
	addNonEmpty("title", n.Title)
	addNonEmpty("body", n.Body)
	addNonEmpty("icon", n.Icon)
	addNonEmpty("badge", n.Badge)
	addNonEmpty("dir", n.Direction)
	addNonEmpty("image", n.Image)
	addNonEmpty("lang", n.Language)
	addTrue("renotify", n.Renotify)
	addTrue("requireInteraction", n.RequireInteraction)
	addTrue("silent", n.Silent)
	addNonEmpty("tag", n.Tag)
	if n.Data != nil {
		m["data"] = n.Data
	}
	if n.TimestampMillis != nil {
		m["timestamp"] = *n.TimestampMillis
	}
	if len(n.Vibrate) > 0 {
		m["vibrate"] = n.Vibrate
	}
	return m
}

// MarshalJSON marshals a WebpushNotification into JSON (for internal use only).
func (n *WebpushNotification) MarshalJSON() ([]byte, error) {
	m := n.standardFields()
	for k, v := range n.CustomData {
		m[k] = v
	}
	return json.Marshal(m)
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
	AlertString      string
	Alert            *ApsAlert
	Badge            *int
	Sound            string
	ContentAvailable bool
	MutableContent   bool
	Category         string
	ThreadID         string
	CustomData       map[string]interface{}
}

// standardFields creates a map containing all the fields except the custom data.
func (a *Aps) standardFields() map[string]interface{} {
	m := make(map[string]interface{})
	if a.Alert != nil {
		m["alert"] = a.Alert
	} else if a.AlertString != "" {
		m["alert"] = a.AlertString
	}
	if a.ContentAvailable {
		m["content-available"] = 1
	}
	if a.MutableContent {
		m["mutable-content"] = 1
	}
	if a.Badge != nil {
		m["badge"] = *a.Badge
	}
	if a.Sound != "" {
		m["sound"] = a.Sound
	}
	if a.Category != "" {
		m["category"] = a.Category
	}
	if a.ThreadID != "" {
		m["thread-id"] = a.ThreadID
	}
	return m
}

// MarshalJSON marshals an Aps into JSON (for internal use only).
func (a *Aps) MarshalJSON() ([]byte, error) {
	m := a.standardFields()
	for k, v := range a.CustomData {
		m[k] = v
	}
	return json.Marshal(m)
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
		op:     iidUnsubscribe,
	}
	return c.makeTopicManagementRequest(ctx, req)
}

// IsInternal checks if the given error was due to an internal server error.
func IsInternal(err error) bool {
	return internal.HasErrorCode(err, internalError)
}

// IsInvalidAPNSCredentials checks if the given error was due to invalid APNS certificate or auth
// key.
func IsInvalidAPNSCredentials(err error) bool {
	return internal.HasErrorCode(err, invalidAPNSCredentials)
}

// IsInvalidArgument checks if the given error was due to an invalid argument in the request.
func IsInvalidArgument(err error) bool {
	return internal.HasErrorCode(err, invalidArgument)
}

// IsMessageRateExceeded checks if the given error was due to the client exceeding a quota.
func IsMessageRateExceeded(err error) bool {
	return internal.HasErrorCode(err, messageRateExceeded)
}

// IsMismatchedCredential checks if the given error was due to an invalid credential or permission
// error.
func IsMismatchedCredential(err error) bool {
	return internal.HasErrorCode(err, mismatchedCredential)
}

// IsRegistrationTokenNotRegistered checks if the given error was due to a registration token that
// became invalid.
func IsRegistrationTokenNotRegistered(err error) bool {
	return internal.HasErrorCode(err, registrationTokenNotRegistered)
}

// IsServerUnavailable checks if the given error was due to the backend server being temporarily
// unavailable.
func IsServerUnavailable(err error) bool {
	return internal.HasErrorCode(err, serverUnavailable)
}

// IsTooManyTopics checks if the given error was due to the client exceeding the allowed number
// of topics.
func IsTooManyTopics(err error) bool {
	return internal.HasErrorCode(err, tooManyTopics)
}

// IsUnknown checks if the given error was due to unknown error returned by the backend server.
func IsUnknown(err error) bool {
	return internal.HasErrorCode(err, unknownError)
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
		Status  string `json:"status"`
		Message string `json:"message"`
		Details []struct {
			Type      string `json:"@type"`
			ErrorCode string `json:"errorCode"`
		}
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
	var serverCode string
	for _, d := range fe.Error.Details {
		if d.Type == "type.googleapis.com/google.firebase.fcm.v1.FcmErrorCode" {
			serverCode = d.ErrorCode
			break
		}
	}
	if serverCode == "" {
		serverCode = fe.Error.Status
	}

	var clientCode, msg string
	info, ok := fcmErrorCodes[serverCode]
	if ok {
		clientCode, msg = info.Code, info.Msg
	} else {
		clientCode = unknownError
		msg = fmt.Sprintf("server responded with an unknown error; response: %s", string(resp.Body))
	}
	if fe.Error.Message != "" {
		msg += "; details: " + fe.Error.Message
	}
	return "", internal.Errorf(clientCode, "http error status: %d; reason: %s", resp.Status, msg)
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
	var clientCode, msg string
	info, ok := iidErrorCodes[ie.Error]
	if ok {
		clientCode, msg = info.Code, info.Msg
	} else {
		clientCode = unknownError
		msg = fmt.Sprintf("client encountered an unknown error; response: %s", string(resp.Body))
	}
	return nil, internal.Errorf(clientCode, "http error status: %d; reason: %s", resp.Status, msg)
}
