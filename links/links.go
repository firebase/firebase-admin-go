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

// Package links contains the dynamic links sdks. Specifically get link stats.
package links

import (
	"bytes"
	"encoding/json"
	"fmt"
	"golang.org/x/net/context"
	"google.golang.org/api/option"
	"google.golang.org/api/transport"
	"net/http"
	"net/url"

	"firebase.google.com/go/internal"
)

const (
	linksEndPoint     = "https://firebasedynamiclinks.googleapis.com/v1/"
	linksStatsRequest = "%s/linkStats?durationDays=%d"
)

// LinkStats is returned from the GetLinkStats, contains an array of Event Stats
type LinkStats struct {
	EventStats []EventStats `json:"linkEventStats"`
}

// EventStats will contain the counts for the aggregations for the requested period
type EventStats struct {
	Platform Platform  `json:"platform"`
	ET       EventType `json:"event"`
	Count    int       `json:"count"`
}

// Platform constant "enum" for the event
type Platform int

// There are 3 possible values for the platforms "enum" in the platform
const (
	UNKNOWNPLATFORM Platform = iota
	DESKTOP
	IOS
	ANDROID
)

var platformID = map[Platform]string{

	DESKTOP: "DESKTOP",
	IOS:     "IOS",
	ANDROID: "ANDROID",
}

var platformName = map[string]Platform{
	"DESKTOP": DESKTOP,
	"IOS":     IOS,
	"ANDROID": ANDROID,
}

func (p Platform) String() string {
	return platformID[p]
}

// MarshalJSON makes the "enum" usable in marshalling json
func (p *Platform) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(platformID[*p])
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

// UnmarshalJSON is used to read the json files into the "enum"
func (p *Platform) UnmarshalJSON(b []byte) error {
	// unmarshal as string
	var s string
	err := json.Unmarshal(b, &s)
	if err != nil {
		return err
	}
	// lookup value
	*p = platformName[s]
	return nil
}

// EventType constant for the event stats
type EventType int

// There are 5 possible values for the event type "enum" in the event_stats
const (
	UNKNOWNEVENT EventType = iota
	CLICK
	REDIRECT
	AppINSTALL
	AppFIRSTOPEN
	AppREOPEN
)

var eventTypesID = map[EventType]string{
	CLICK:        "CLICK",
	REDIRECT:     "REDIRECT",
	AppINSTALL:   "APP_INSTALL",
	AppFIRSTOPEN: "APP_FIRST_OPEN",
	AppREOPEN:    "APP_RE_OPEN",
}

var eventTypesName = map[string]EventType{
	"CLICK":          CLICK,
	"REDIRECT":       REDIRECT,
	"APP_INSTALL":    AppINSTALL,
	"APP_FIRST_OPEN": AppFIRSTOPEN,
	"APP_RE_OPEN":    AppREOPEN,
}

func (e EventType) String() string {
	return eventTypesID[e]
}

// MarshalJSON makes the "enum" usable in marshalling json
func (e *EventType) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(eventTypesID[*e])
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

// UnmarshalJSON is used to read the json files into the "enum"
func (e *EventType) UnmarshalJSON(b []byte) error {
	// unmarshal as string
	var s string
	err := json.Unmarshal(b, &s)
	if err != nil {
		return err
	}
	// lookup value
	*e = eventTypesName[s]
	return nil
}

// StatOptions are used in the request for GetLinkStats
type StatOptions struct {
	DurationDays int
}

// Client is the interface for the Firebase dynamics links service.
//
// Client is the entry point to the dynamic links functions
type Client struct {
	hc                *internal.HTTPClient
	linksEndPoint     string
	linksStatsRequest string
}

// NewClient creates a new instance of the Firebase Auth Client.
//
// This function can only be invoked from within the SDK. Client applications should access the
// Auth service through firebase.App.
func NewClient(ctx context.Context, opts ...option.ClientOption) (*Client, error) {
	hc, _, err := transport.NewHTTPClient(ctx, opts...)
	if err != nil {
		return nil, err
	}

	return &Client{
		hc:                &internal.HTTPClient{Client: hc},
		linksEndPoint:     linksEndPoint,
		linksStatsRequest: linksStatsRequest,
	}, nil
}

// LinkStats returns the link stats given a url, and the duration (inside the StatOptions)
func (c *Client) LinkStats(ctx context.Context, shortLink string, statOptions StatOptions) (*LinkStats, error) {
	if err := validateShortLink(shortLink); err != nil {
		return nil, err
	}
	if err := validateStatOptions(statOptions); err != nil {
		return nil, err
	}
	request := &internal.Request{
		Method: http.MethodGet,
		URL:    c.makeURLForLinkStats(shortLink, statOptions),
	}

	_ = request
	return nil, nil
}

func (c *Client) makeURLForLinkStats(shortLink string, statOptions StatOptions) string {
	return fmt.Sprintf(c.linksEndPoint+c.linksStatsRequest,
		url.QueryEscape(shortLink),
		statOptions.DurationDays)
}

func validateShortLink(shortLink string) error {
	return nil
}

func validateStatOptions(statOptions StatOptions) error {
	return nil
}
