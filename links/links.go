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
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/context"

	"firebase.google.com/go/internal"
	"google.golang.org/api/option"
	"google.golang.org/api/transport"
)

const (
	linksEndPoint     = "https://firebasedynamiclinks.googleapis.com/v1"
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
	Count    int32     `json:"count,string"`
}

// Platform constant "enum" for the event
type Platform int

// There are 3 possible values for the platforms "enum" in the platform
const (
	_ Platform = iota
	Desktop
	IOS
	Android
)

var platformByID = map[Platform]string{
	Desktop: "DESKTOP",
	IOS:     "IOS",
	Android: "ANDROID",
}

var platformByName = map[string]Platform{
	"DESKTOP": Desktop,
	"IOS":     IOS,
	"ANDROID": Android,
}

func (p Platform) String() string {
	return platformByID[p]
}

// MarshalJSON makes the "enum" usable in marshalling json
func (p *Platform) MarshalJSON() ([]byte, error) {
	return []byte(`"` + platformByID[*p] + `"`), nil
}

// UnmarshalJSON is used to read the json files into the "enum"
func (p *Platform) UnmarshalJSON(b []byte) error {
	var s string
	err := json.Unmarshal(b, &s)
	if err != nil {
		return err
	}
	var ok bool
	if *p, ok = platformByName[s]; !ok {
		return fmt.Errorf("unknown platform %q", s)
	}
	return nil
}

// EventType constant for the event stats
type EventType int

// There are 5 possible values for the event type "enum" in the event_stats
const (
	_ EventType = iota
	Click
	Redirect
	AppInstall
	AppFirstOpen
	AppReOpen
)

var eventTypesByID = map[EventType]string{
	Click:        "CLICK",
	Redirect:     "REDIRECT",
	AppInstall:   "APP_INSTALL",
	AppFirstOpen: "APP_FIRST_OPEN",
	AppReOpen:    "APP_RE_OPEN",
}

var eventTypesByName = map[string]EventType{
	"CLICK":          Click,
	"REDIRECT":       Redirect,
	"APP_INSTALL":    AppInstall,
	"APP_FIRST_OPEN": AppFirstOpen,
	"APP_RE_OPEN":    AppReOpen,
}

func (e EventType) String() string {
	return eventTypesByID[e]
}

// MarshalJSON makes the "enum" usable in marshalling json
func (e *EventType) MarshalJSON() ([]byte, error) {
	return []byte(`"` + eventTypesByID[*e] + `"`), nil
}

// UnmarshalJSON is used to read the json files into the "enum"
func (e *EventType) UnmarshalJSON(b []byte) error {
	var s string
	err := json.Unmarshal(b, &s)
	if err != nil {
		return err
	}
	var ok bool
	if *e, ok = eventTypesByName[s]; !ok {
		return fmt.Errorf("unknown event type %q", s)
	}
	return nil
}

// StatOptions are used in the request for GetLinkStats, it is used to request data
// going back DurationDays days.
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

// LinkStats returns the link stats given a shortLink, and the duration (days, inside the StatOptions)
// Returns a LinkStats object which contains a list of EventStats.
// The service account with which the firebase_app is validated must be associated with the project
// for which the stats are requested.
// If the URI prefix for the shortlink belongs to the project but the link suffix has either not
// been created or has no data in the requested period, the LinkStats object will contain an
// empty list of EventStats.
func (c *Client) LinkStats(ctx context.Context, shortLink string, statOptions StatOptions) (*LinkStats, error) {
	if ok := strings.HasPrefix(shortLink, "https://"); !ok {
		return nil, fmt.Errorf("short link must start with `https://`")
	}
	if ok := statOptions.DurationDays > 0; !ok {
		return nil, fmt.Errorf("durationDays must be > 0")
	}
	request := &internal.Request{
		Method: http.MethodGet,
		URL:    c.makeURLForLinkStats(shortLink, statOptions),
	}

	resp, err := c.hc.Do(ctx, request)
	if err != nil {
		return nil, err
	}
	if err := resp.CheckStatus(http.StatusOK); err != nil {
		return nil, err
	}
	var result LinkStats
	err = json.Unmarshal(resp.Body, &result)
	return &result, err
}

func (c *Client) makeURLForLinkStats(shortLink string, statOptions StatOptions) string {
	return fmt.Sprintf(c.linksEndPoint+"/"+c.linksStatsRequest,
		url.QueryEscape(shortLink),
		statOptions.DurationDays)
}
