package remoteconfig

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"firebase.google.com/go/v4/internal"
)

const (
	defaultRemoteConfigEndpoint = "https://firebaseremoteconfig.googleapis.com/v1"
)

// Client .
type Client struct {
	hc        *internal.HTTPClient
	projectID string
}

func (c *Client) getRootURL() string {
	return fmt.Sprintf("%s/projects/%s/remoteConfig", defaultRemoteConfigEndpoint, c.projectID)
}

// GetRemoteConfig https://firebase.google.com/docs/reference/remote-config/rest/v1/projects/getRemoteConfig
func (c *Client) GetRemoteConfig(versionNumber string) (*Response, error) {
	var opts []internal.HTTPOption

	// Optional. Version number of the RemoteConfig to look up.
	// If not specified, the latest RemoteConfig will be returned.
	if versionNumber != "" {
		opts = append(opts, internal.WithQueryParam("versionNumber", versionNumber))
	}

	var data RemoteConfig

	resp, err := c.hc.DoAndUnmarshal(
		context.Background(),
		&internal.Request{
			Method: http.MethodGet,
			URL:    c.getRootURL(),
			Opts:   opts,
		},
		&data,
	)
	if err != nil {
		return nil, err
	}

	result := &Response{
		RemoteConfig: data,
		Etag:         resp.Header.Get("Etag"),
	}

	return result, nil
}

// UpdateRemoteConfig https://firebase.google.com/docs/reference/remote-config/rest/v1/projects/updateRemoteConfig
func (c *Client) UpdateRemoteConfig(eTag string, validateOnly bool) (*Response, error) {
	if eTag == "" {
		eTag = "*"
	}

	var opts []internal.HTTPOption
	opts = append(opts, internal.WithHeader("If-Match", eTag))
	opts = append(opts, internal.WithQueryParam("validateOnly", strconv.FormatBool(validateOnly)))

	var data RemoteConfig

	resp, err := c.hc.DoAndUnmarshal(
		context.Background(),
		&internal.Request{
			Method: http.MethodPut,
			URL:    c.getRootURL(),
			Opts:   opts,
		},
		&data,
	)
	if err != nil {
		return nil, err
	}

	result := &Response{
		RemoteConfig: data,
		Etag:         resp.Header.Get("Etag"),
	}

	return result, nil
}

// ListVersions https://firebase.google.com/docs/reference/remote-config/rest/v1/projects.remoteConfig/listVersions
func (c *Client) ListVersions(options *ListVersionsOptions) (*ListVersionsResponse, error) {
	var opts []internal.HTTPOption
	if options.PageSize != 0 {
		opts = append(opts, internal.WithQueryParam("pageSize", strconv.Itoa(options.PageSize)))
	}

	if options.PageToken != "" {
		opts = append(opts, internal.WithQueryParam("pageToken", options.PageToken))
	}

	if options.EndVersionNumber != "" {
		opts = append(opts, internal.WithQueryParam("endVersionNumber", options.EndVersionNumber))
	}

	if !options.StartTime.IsZero() {
		opts = append(opts, internal.WithQueryParam("startTime", options.StartTime.Format(time.RFC3339Nano)))
	}

	if !options.EndTime.IsZero() {
		opts = append(opts, internal.WithQueryParam("endTime", options.EndTime.Format(time.RFC3339Nano)))
	}

	var data ListVersionsResponse
	url := fmt.Sprintf("%s:listVersions", c.getRootURL())

	_, err := c.hc.DoAndUnmarshal(
		context.Background(),
		&internal.Request{
			Method: http.MethodGet,
			URL:    url,
			Opts:   opts,
		},
		&data,
	)
	if err != nil {
		return nil, err
	}

	return &data, nil
}

// Rollback will perform a rollback operation on the template
// https://firebase.google.com/docs/reference/remote-config/rest/v1/projects.remoteConfig/rollback
func (c *Client) Rollback(ctx context.Context, versionNumber string) (*Template, error) {
	if versionNumber == "" {
		return nil, errors.New("versionNumber is required to rollback a Remote Config template")
	}

	var data Template
	url := fmt.Sprintf("%s:rollback", c.getRootURL())

	_, err := c.hc.DoAndUnmarshal(
		context.Background(),
		&internal.Request{
			Method: http.MethodPost,
			Body: internal.NewJSONEntity(
				struct {
					VersionNumber string `json:"versionNumber"`
				}{
					VersionNumber: versionNumber,
				},
			),
			URL: url,
		},
		&data,
	)
	if err != nil {
		return nil, err
	}

	return &data, nil
}

// NewClient returns the default remote config
func NewClient(ctx context.Context, c *internal.RemoteConfig) (*Client, error) {
	if c.ProjectID == "" {
		return nil, errors.New("project ID is required to access Firebase Cloud Remote Config client")
	}

	hc, _, err := internal.NewHTTPClient(ctx, c.Opts...)
	if err != nil {
		return nil, err
	}

	return &Client{
		hc:        hc,
		projectID: c.ProjectID,
	}, nil
}

// GetTemplate will retrieve the latest template
func (c *Client) GetTemplate(ctx context.Context) (*Template, error) {
	// TODO
	return nil, nil
}

// GetTemplateAtVersion will retrieve the specified version of the template
func (c *Client) GetTemplateAtVersion(ctx context.Context, versionNumber string) (*Template, error) {
	// TODO
	return nil, nil
}

// Versions will list the versions of the template
func (c *Client) Versions(ctx context.Context, options *ListVersionsOptions) (*VersionIterator, error) {
	// TODO
	return nil, nil
}

// PublishTemplate will publish the specified temnplate
func (c *Client) PublishTemplate(ctx context.Context, template *Template) (*Template, error) {
	// TODO
	return nil, nil
}

// ValidateTemplate will run validations for the current template
func (c *Client) ValidateTemplate(ctx context.Context, template *Template) (*Template, error) {
	// TODO
	return nil, nil
}

// ForcePublishTemplate will publish the template irrespective of the outcome from validations
func (c *Client) ForcePublishTemplate(ctx context.Context, template *Template) (*Template, error) {
	// TODO
	return nil, nil
}
