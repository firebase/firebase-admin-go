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

// Package links contains a function for retrieving the stats for a short
// dynamic link.
package links

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/context"

	"firebase.google.com/go/internal"
	"google.golang.org/api/transport"
)

const (
	linksEndpoint     = "https://firebasedynamiclinks.googleapis.com/v1"
	linksStatsRequest = "%s/linkStats?durationDays=%d"
)

// LinkStats is returned from the GetLinkStats, contains an array of Event Stats
type LinkStats struct {
	EventStats []EventStats `json:"linkEventStats"`
}

// EventStats will contain the counts for the aggregations for the requested period
type EventStats struct {
	Platform  Platform  `json:"platform"`
	EventType EventType `json:"event"`
	Count     int32     `json:"count,string"`
}

// Platform constant "enum" for the event
type Platform string

// There are 3 possible values for the platforms "enum" in the platform
const (
	Desktop Platform = "DESKTOP"
	IOS     Platform = "IOS"
	Android Platform = "ANDROID"
)

// EventType constant for the event stats
type EventType string

// There are 5 possible values for the event type "enum" in the event_stats
const (
	Click        EventType = "CLICK"
	Redirect     EventType = "REDIRECT"
	AppInstall   EventType = "APP_INSTALL"
	AppFirstOpen EventType = "APP_FIRST_OPEN"
	AppReOpen    EventType = "APP_RE_OPEN"
)

// StatOptions are used in the request for GetLinkStats. It is used to request data
// covering the last DurationDays days.
type StatOptions struct {
	DurationDays int
}

// Client is the interface for the Firebase dynamics links service.
//
// Client is the entry point to the dynamic links functions
type Client struct {
	hc                *internal.HTTPClient
	linksEndpoint     string
	linksStatsRequest string
}

// NewClient creates a new instance of the Firebase Dynamic Links Client.
//
// This function can only be invoked from within the SDK. Client applications should access the
// Dynamic Links service through firebase.App.
func NewClient(ctx context.Context, c *internal.LinksConfig) (*Client, error) {
	hc, _, err := transport.NewHTTPClient(ctx, c.Opts...)
	if err != nil {
		return nil, err
	}

	return &Client{
		hc:                &internal.HTTPClient{Client: hc},
		linksEndpoint:     linksEndpoint,
		linksStatsRequest: linksStatsRequest,
	}, nil
}

// LinkStats returns the stats given a shortLink, and the duration (days, inside the StatOptions)
//
// Returns a LinkStats object which contains a list of EventStats.
// The credential with which the firebase.App is initialized must be associated with the project
// for which the stats are requested.
// If the URI prefix for the shortlink belongs to the project but the link suffix has either not
// been created or has no data in the requested period, the LinkStats object will contain an
// empty list of EventStats.
func (c *Client) LinkStats(ctx context.Context, shortLink string, statOptions StatOptions) (*LinkStats, error) {
	if !strings.HasPrefix(shortLink, "https://") {
		return nil, fmt.Errorf("short link must start with `https://`")
	}
	if statOptions.DurationDays <= 0 {
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
	if err = json.Unmarshal(resp.Body, &result); err != nil {
		return nil, err
	}
	return &result, err
}

func (c *Client) makeURLForLinkStats(shortLink string, statOptions StatOptions) string {
	return fmt.Sprintf(c.linksEndpoint+"/"+c.linksStatsRequest,
		url.QueryEscape(shortLink),
		statOptions.DurationDays)
}
