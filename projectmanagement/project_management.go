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

// Package projectmanagement contains functions for programmatic setup and management of Firebase projects.
package projectmanagement

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"time"

	"firebase.google.com/go/v4/internal"
)

const (
	baseEndpoint            = "https://firebase.googleapis.com/v1beta1"
	defaultProjectsEndpoint = baseEndpoint + "/projects"
)

// State represents an App state
type State string

const (
	StateUnspecified State = "STATE_UNSPECIFIED"
	StateActive      State = "ACTIVE"
	StateDeleted     State = "DELETED"
)

type resourceType string

const (
	resourceWebApps     resourceType = "webApps"
	resourceIosApps     resourceType = "iosApps"
	resourceAndroidApps resourceType = "androidApps"
)

// AndroidApp represents a Android Application inside a Firebase project
type AndroidApp struct {
	Name         string   `json:"name,omitempty"`
	AppID        string   `json:"appId,omitempty"`
	DisplayName  string   `json:"displayName,omitempty"`
	ProjectId    string   `json:"projectId,omitempty"`
	PackageName  string   `json:"packageName,omitempty"`
	ApiKeyId     string   `json:"apiKeyId,omitempty"`
	State        State    `json:"state,omitempty"`
	Sha1Hashes   []string `json:"sha1Hashes,omitempty"`
	Sha256Hashes []string `json:"sha256Hashes,omitempty"`
	ETag         string   `json:"etag,omitempty"`
}

// AndroidAppConfig represents the configuration for a Android Application inside a Firebase project
type AndroidAppConfig struct {
	ConfigFileName     string `json:"configFileName,omitempty"`
	ConfigFileContents string `json:"configFileContents,omitempty"`
}

// IosApp represents a iOS Application inside a Firebase project
type IosApp struct {
	Name       string `json:"name,omitempty"`
	AppID      string `json:"appId,omitempty"`
	ProjectId  string `json:"projectId,omitempty"`
	BundleId   string `json:"bundleId,omitempty"`
	AppStoreId string `json:"appStoreId,omitempty"`
	TeamId     string `json:"teamId,omitempty"`
	ApiKeyId   string `json:"apiKeyId,omitempty"`
	State      State  `json:"state,omitempty"`
	ETag       string `json:"etag,omitempty"`
}

// IosAppConfig represents the configuration for a iOS Application inside a Firebase project
type IosAppConfig struct {
	ConfigFileName     string `json:"configFileName,omitempty"`
	ConfigFileContents string `json:"configFileContents,omitempty"`
}

// WebApp represents a Web Application inside a Firebase project
type WebApp struct {
	Name          string   `json:"name,omitempty"`
	AppID         string   `json:"appId,omitempty"`
	ProjectNumber string   `json:"projectNumber,omitempty"`
	DisplayName   string   `json:"displayName,omitempty"`
	AppUrls       []string `json:"appUrls,omitempty"`
	WebId         string   `json:"web_id,omitempty"`
	ApiKeyId      string   `json:"apiKeyId,omitempty"`
	State         State    `json:"state,omitempty"`
	ETag          string   `json:"etag,omitempty"`
}

// WebAppConfig represents the configuration for a Web Application inside a Firebase project
type WebAppConfig struct {
	ProjectID         string `json:"projectId,omitempty"`
	AppID             string `json:"appId,omitempty"`
	DatabaseURL       string `json:"databaseUrl,omitempty"`
	StorageBucket     string `json:"storageBucket,omitempty"`
	LocationId        string `json:"locationId,omitempty"`
	ApiKey            string `json:"apiKey,omitempty"`
	AuthDomain        string `json:"authDomain,omitempty"`
	MessagingSenderId string `json:"messagingSenderId,omitempty"`
	MeasurementId     string `json:"measurementId,omitempty"`
}

// Client is the interface for the Firebase Management service.
type Client struct {
	projectsEndpoint string
	project          string
	httpClient       *internal.HTTPClient
}

// NewClient creates a new instance of the Firebase Projects Client.
//
// This function can only be invoked from within the SDK. Client applications should access the
// the messaging service through firebase.App.
func NewClient(ctx context.Context, c *internal.ProjectManagementConfig) (*Client, error) {
	if c.ProjectID == "" {
		return nil, errors.New("project ID is required to access Firebase Cloud Messaging client")
	}

	hc, projectsEndpoint, err := internal.NewHTTPClient(ctx, c.Opts...)
	if err != nil {
		return nil, err
	}

	if projectsEndpoint == "" {
		projectsEndpoint = defaultProjectsEndpoint
	}

	return &Client{
		projectsEndpoint: projectsEndpoint,
		project:          c.ProjectID,
		httpClient:       hc,
	}, nil
}

/////////////////////////////////////////
// AndroidApps

// ListIosApps lists each IosApp associated with the specified FirebaseProject.
func (c *Client) ListAndroidApps(ctx context.Context) ([]*AndroidApp, error) {
	return listApps[AndroidApp](ctx, c, string(resourceAndroidApps))
}

// GetIosApp gets the IosApp identified by an appId.
func (c *Client) GetAndroidApp(ctx context.Context, appId string) (*AndroidApp, error) {
	return getApp[AndroidApp](ctx, c, string(resourceAndroidApps), appId)
}

// GetIosAppConfig gets the configuration artifact associated with the specified AndroidApp.
func (c *Client) GetAndroidAppConfig(ctx context.Context, app *AndroidApp) (*AndroidAppConfig, error) {
	return getAppConfig[AndroidAppConfig](ctx, c, string(resourceAndroidApps), app.AppID)
}

// CreateIosApp creates a new AndroidApp in the project.
func (c *Client) CreateAndroidApp(ctx context.Context, packageName string) (*AndroidApp, error) {
	app := &AndroidApp{
		PackageName: packageName,
	}
	return createApp(ctx, c, string(resourceAndroidApps), app)
}

// UpdateIosApp updates an existing AndroidApp in the project.
func (c *Client) UpdateAndroidApp(ctx context.Context, app *AndroidApp) (*AndroidApp, error) {
	return updateApp(ctx, c, string(resourceAndroidApps), app.AppID, app)
}

// RemoveIosApp removes the AndroidApp form the Firebase project.
func (c *Client) RemoveAndroidApp(ctx context.Context, app *AndroidApp, immediate bool) error {
	return removeApp[AndroidApp](ctx, c, string(resourceAndroidApps), app.AppID, immediate)
}

/////////////////////////////////////////
// IosApps

// ListIosApps lists each IosApp associated with the specified FirebaseProject.
func (c *Client) ListIosApps(ctx context.Context) ([]*IosApp, error) {
	return listApps[IosApp](ctx, c, string(resourceIosApps))
}

// GetIosApp gets the IosApp identified by an appId.
func (c *Client) GetIosApp(ctx context.Context, appId string) (*IosApp, error) {
	return getApp[IosApp](ctx, c, string(resourceIosApps), appId)
}

// GetIosAppConfig gets the configuration artifact associated with the specified IosApp.
func (c *Client) GetIosAppConfig(ctx context.Context, app *IosApp) (*IosAppConfig, error) {
	return getAppConfig[IosAppConfig](ctx, c, string(resourceIosApps), app.AppID)
}

// CreateIosApp creates a new IosApp in the project.
func (c *Client) CreateIosApp(ctx context.Context, bundleId string) (*IosApp, error) {
	app := &IosApp{
		BundleId: bundleId,
	}
	return createApp(ctx, c, string(resourceIosApps), app)
}

// UpdateIosApp updates an existing IosApp in the project.
func (c *Client) UpdateIosApp(ctx context.Context, app *IosApp) (*IosApp, error) {
	return updateApp(ctx, c, string(resourceIosApps), app.AppID, app)
}

// RemoveIosApp removes the IosApp form the Firebase project.
func (c *Client) RemoveIosApp(ctx context.Context, app *IosApp, immediate bool) error {
	return removeApp[IosApp](ctx, c, string(resourceIosApps), app.AppID, immediate)
}

/////////////////////////////////////////
// WebApps

// ListWebApps lists each WebApp associated with the specified FirebaseProject.
func (c *Client) ListWebApps(ctx context.Context) ([]*WebApp, error) {
	return listApps[WebApp](ctx, c, string(resourceWebApps))
}

// GetWebApp gets the WebApp identified by an appId.
func (c *Client) GetWebApp(ctx context.Context, appId string) (*WebApp, error) {
	return getApp[WebApp](ctx, c, string(resourceWebApps), appId)
}

// RemoveWebApp removes the WebApp form the Firebase project.
func (c *Client) RemoveWebApp(ctx context.Context, app *WebApp, immediate bool) error {
	return removeApp[WebApp](ctx, c, string(resourceWebApps), app.AppID, immediate)
}

// CreateWebApp creates a new WebApp in the project.
func (c *Client) CreateWebApp(ctx context.Context, displayName string) (*WebApp, error) {
	webApp := &WebApp{
		DisplayName: displayName,
	}
	return createApp(ctx, c, string(resourceWebApps), webApp)
}

// UpdateWebApp updates an existing WebApp in the project.
func (c *Client) UpdateWebApp(ctx context.Context, app *WebApp) (*WebApp, error) {
	return updateApp(ctx, c, string(resourceWebApps), app.AppID, app)
}

// GetWebAppConfig gets the configuration artifact associated with the specified WebApp.
func (c *Client) GetWebAppConfig(ctx context.Context, app *WebApp) (*WebAppConfig, error) {
	return getAppConfig[WebAppConfig](ctx, c, string(resourceWebApps), app.AppID)
}

// //////////////////////////////////
// Operations
const (
	// Operations polling params
	pollingMaximumAttempts          = 8
	pollingBaseWaitTimeseconds      = 0.5
	pollingExponentialBackoffFactor = 1.5
)

var (
	errOperationNoResult      = errors.New("operation done without results")
	errOperationNotDoneInTime = errors.New("operation deadline exceeded")
	errOperationInvalidName   = errors.New("empty operation name")
)

type opStatus struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type operation struct {
	Name     string           `json:"name,omitempty"`
	Metadata json.RawMessage  `json:"metadata,omitempty"`
	Done     bool             `json:"done,omitempty"`
	Error    *opStatus        `json:"error,omitempty"`
	Response *json.RawMessage `json:"response,omitempty"`
}

func (c *Client) pollOperation(ctx context.Context, operationName string) (*json.RawMessage, error) {
	if operationName == "" {
		return nil, errOperationInvalidName
	}

	for n := 0; n < pollingMaximumAttempts; n++ {
		delayFactor := math.Pow(pollingExponentialBackoffFactor, float64(n))
		waitTime := time.Duration(delayFactor * pollingBaseWaitTimeseconds * 1000 * 1000 * 1000)
		time.Sleep(waitTime)

		request := &internal.Request{
			Method: http.MethodGet,
			URL:    fmt.Sprintf("%s/%s", baseEndpoint, operationName),
		}

		var op operation
		_, err := c.httpClient.DoAndUnmarshal(ctx, request, &op)
		if err != nil {
			return nil, err
		}

		if op.Done {
			if op.Error != nil {
				return nil, fmt.Errorf("operation error (%d, %s)", op.Error.Code, op.Error.Message)
			}
			if op.Response != nil {
				return op.Response, nil
			}
			return nil, errOperationNoResult
		}
	}
	return nil, errOperationNotDoneInTime
}

func (c *Client) pollOperationAndUnmarshal(ctx context.Context, operationName string, v any) error {
	response, err := c.pollOperation(ctx, operationName)
	if err != nil {
		return err
	}
	return json.Unmarshal(*response, v)
}

// //////////////////////////////////
// Generic functions
func listApps[T any](ctx context.Context, c *Client, resource string) ([]*T, error) {
	request := &internal.Request{
		Method: http.MethodGet,
		URL:    fmt.Sprintf("%s/%s/%s", c.projectsEndpoint, c.project, resource),
	}

	response := struct {
		Apps          []*T   `json:"apps,omitempty"`
		NextPageToken string `json:"nextPageToken,omitempty"`
	}{}

	_, err := c.httpClient.DoAndUnmarshal(ctx, request, &response)
	return response.Apps, err
}

func getApp[T any](ctx context.Context, c *Client, resource, identifier string) (*T, error) {
	request := &internal.Request{
		Method: http.MethodGet,
		URL:    fmt.Sprintf("%s/-/%s/%s", c.projectsEndpoint, resource, identifier),
	}

	var result T
	_, err := c.httpClient.DoAndUnmarshal(ctx, request, &result)
	return &result, err
}

func getAppConfig[T any](ctx context.Context, c *Client, resource, identifier string) (*T, error) {
	request := &internal.Request{
		Method: http.MethodGet,
		URL:    fmt.Sprintf("%s/-/%s/%s/config", c.projectsEndpoint, resource, identifier),
	}

	var result T
	_, err := c.httpClient.DoAndUnmarshal(ctx, request, &result)
	return &result, err
}

func createApp[T any](ctx context.Context, c *Client, resource string, app *T) (*T, error) {
	request := &internal.Request{
		Method: http.MethodPost,
		URL:    fmt.Sprintf("%s/%s/%s", c.projectsEndpoint, c.project, resource),
		Body:   internal.NewJSONEntity(app),
	}

	var op operation
	if _, err := c.httpClient.DoAndUnmarshal(ctx, request, &op); err != nil {
		return nil, err
	}

	if err := c.pollOperationAndUnmarshal(ctx, op.Name, app); err != nil {
		return nil, err
	}
	return app, nil
}

func updateApp[T any](ctx context.Context, c *Client, resource, identifier string, app *T) (*T, error) {
	request := &internal.Request{
		Method: http.MethodPatch,
		URL:    fmt.Sprintf("%s/%s/%s/%s", c.projectsEndpoint, c.project, resource, identifier),
		Body:   internal.NewJSONEntity(app),
	}

	if _, err := c.httpClient.DoAndUnmarshal(ctx, request, app); err != nil {
		return nil, err
	}
	return app, nil
}

func removeApp[T any](ctx context.Context, c *Client, resource, identifier string, immediate bool) error {
	body := struct {
		Immediate bool `json:"immediate,omitempty"`
	}{
		Immediate: immediate,
	}

	request := &internal.Request{
		Method: http.MethodPost,
		URL:    fmt.Sprintf("%s/-/%s/%s:remove", c.projectsEndpoint, resource, identifier),
		Body:   internal.NewJSONEntity(body),
	}

	var op operation
	if _, err := c.httpClient.DoAndUnmarshal(ctx, request, &op); err != nil {
		return err
	}

	if !op.Done && op.Name != "" {
		_, err := c.pollOperation(ctx, op.Name)
		return err
	}
	return nil
}
