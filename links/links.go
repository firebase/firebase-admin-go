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

// Package links contains a function for retrieving the statistics for a short
// dynamic link.
package links

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/context"

	"firebase.google.com/go/internal"
	"google.golang.org/api/transport"
)

// Platform constant "enum" for the event
type Platform string

// EventType constant for the event stats
type EventType string

const (
	// Desktop platform type.
	Desktop Platform = "DESKTOP"

	// IOS platform type.
	IOS Platform = "IOS"

	// Android platform type.
	Android Platform = "ANDROID"

	// Click event type.
	Click EventType = "CLICK"

	// Redirect event type.
	Redirect EventType = "REDIRECT"

	// AppInstall event type.
	AppInstall EventType = "APP_INSTALL"

	// AppFirstOpen event type.
	AppFirstOpen EventType = "APP_FIRST_OPEN"

	// AppReOpen event type.
	AppReOpen EventType = "APP_RE_OPEN"

	linksEndpoint = "https://firebasedynamiclinks.googleapis.com/v1"
)

// EventStats will contain the aggregated counts for the requested period.
type EventStats struct {
	Platform  Platform  `json:"platform"`
	EventType EventType `json:"event"`
	Count     int32     `json:"count,string"`
}

// LinkStats contains an array of EventStats.
type LinkStats struct {
	EventStats []EventStats `json:"linkEventStats"`
}

// StatOptions are used in the request for GetLinkStats. It is used to request data
// covering the last N days.
type StatOptions struct {
	LastNDays int
}

// Client is the interface for the Firebase Dynamics Links (FDL) service.
type Client struct {
	client        *internal.HTTPClient
	linksEndpoint string
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
		client:        &internal.HTTPClient{Client: hc},
		linksEndpoint: linksEndpoint,
	}, nil
}

// LinkStats returns the stats given a short link and a duration (last n days).
//
// If the URI prefix for the shortlink belongs to the project but the link suffix has either not
// been created or has no data in the requested period, the returned LinkStats object will contain
// an empty list of EventStats.
func (c *Client) LinkStats(
	ctx context.Context, shortLink string, statOptions StatOptions) (*LinkStats, error) {

	if !strings.HasPrefix(shortLink, "https://") {
		return nil, errors.New("short link must start with https://")
	}
	if statOptions.LastNDays <= 0 {
		return nil, errors.New("last n days must be positive")
	}
	request := &internal.Request{
		Method: http.MethodGet,
		URL: fmt.Sprintf("%s/%s/linkStats?durationDays=%d",
			c.linksEndpoint, url.QueryEscape(shortLink), statOptions.LastNDays),
	}

	resp, err := c.client.Do(ctx, request)
	if err != nil {
		return nil, err
	}
	var result LinkStats
	if err = resp.Unmarshal(http.StatusOK, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
