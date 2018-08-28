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

// Package iid contains functions for deleting instance IDs from Firebase projects.
package iid // import "firebase.google.com/go/iid"

import (
	"errors"
	"fmt"
	"net/http"

	"golang.org/x/net/context"

	"google.golang.org/api/transport"

	"firebase.google.com/go/internal"
)

const iidEndpoint = "https://console.firebase.google.com/v1"

const (
	invalidArgument        = "invalid-argument"
	unauthorized           = "unauthorized"
	insufficientPermission = "insufficient-permission"
	notFound               = "instance-id-not-found"
	alreadyDeleted         = "instance-id-already-deleted"
	tooManyRequests        = "too-many-requests"
	internalError          = "internal-error"
	serverUnavailable      = "server-unavailable"
	unknown                = "unknown-error"
)

var errorCodes = map[int]struct {
	code, message string
}{
	http.StatusBadRequest:   {invalidArgument, "malformed instance id argument"},
	http.StatusUnauthorized: {insufficientPermission, "request not authorized"},
	http.StatusForbidden: {
		insufficientPermission,
		"project does not match instance ID or the client does not have sufficient privileges",
	},
	http.StatusNotFound:            {notFound, "failed to find the instance id"},
	http.StatusConflict:            {alreadyDeleted, "already deleted"},
	http.StatusTooManyRequests:     {tooManyRequests, "request throttled out by the backend server"},
	http.StatusInternalServerError: {internalError, "internal server error"},
	http.StatusServiceUnavailable:  {serverUnavailable, "backend servers are over capacity"},
}

// IsInvalidArgument checks if the given error was due to an invalid instance ID argument.
func IsInvalidArgument(err error) bool {
	return internal.HasErrorCode(err, invalidArgument)
}

// IsInsufficientPermission checks if the given error was due to the request not having the
// required authorization. This could be due to the client not having the required permission
// or the specified instance ID not matching the target Firebase project.
func IsInsufficientPermission(err error) bool {
	return internal.HasErrorCode(err, insufficientPermission)
}

// IsNotFound checks if the given error was due to a non existing instance ID.
func IsNotFound(err error) bool {
	return internal.HasErrorCode(err, notFound)
}

// IsAlreadyDeleted checks if the given error was due to the instance ID being already deleted from
// the project.
func IsAlreadyDeleted(err error) bool {
	return internal.HasErrorCode(err, alreadyDeleted)
}

// IsTooManyRequests checks if the given error was due to the client sending too many requests
// causing a server quota to exceed.
func IsTooManyRequests(err error) bool {
	return internal.HasErrorCode(err, tooManyRequests)
}

// IsInternal checks if the given error was due to an internal server error.
func IsInternal(err error) bool {
	return internal.HasErrorCode(err, internalError)
}

// IsServerUnavailable checks if the given error was due to the backend server being temporarily
// unavailable.
func IsServerUnavailable(err error) bool {
	return internal.HasErrorCode(err, serverUnavailable)
}

// IsUnknown checks if the given error was due to unknown error returned by the backend server.
func IsUnknown(err error) bool {
	return internal.HasErrorCode(err, unknown)
}

// Client is the interface for the Firebase Instance ID service.
type Client struct {
	// To enable testing against arbitrary endpoints.
	endpoint string
	client   *internal.HTTPClient
	project  string
}

// NewClient creates a new instance of the Firebase instance ID Client.
//
// This function can only be invoked from within the SDK. Client applications should access the
// the instance ID service through firebase.App.
func NewClient(ctx context.Context, c *internal.InstanceIDConfig) (*Client, error) {
	if c.ProjectID == "" {
		return nil, errors.New("project id is required to access instance id client")
	}

	hc, _, err := transport.NewHTTPClient(ctx, c.Opts...)
	if err != nil {
		return nil, err
	}

	return &Client{
		endpoint: iidEndpoint,
		client:   &internal.HTTPClient{Client: hc},
		project:  c.ProjectID,
	}, nil
}

// DeleteInstanceID deletes the specified instance ID and the associated data from Firebase..
//
// Note that Google Analytics for Firebase uses its own form of Instance ID to keep track of
// analytics data. Therefore deleting a regular instance ID does not delete Analytics data.
// See https://firebase.google.com/support/privacy/manage-iids#delete_an_instance_id for
// more information.
func (c *Client) DeleteInstanceID(ctx context.Context, iid string) error {
	if iid == "" {
		return errors.New("instance id must not be empty")
	}

	url := fmt.Sprintf("%s/project/%s/instanceId/%s", c.endpoint, c.project, iid)
	resp, err := c.client.Do(ctx, &internal.Request{Method: http.MethodDelete, URL: url})
	if err != nil {
		return err
	}

	if info, ok := errorCodes[resp.Status]; ok {
		return internal.Errorf(info.code, "instance id %q: %s", iid, info.message)
	}
	if err := resp.CheckStatus(http.StatusOK); err != nil {
		return internal.Error(unknown, err.Error())
	}
	return nil
}
