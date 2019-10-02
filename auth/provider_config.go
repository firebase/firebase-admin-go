// Copyright 2019 Google Inc. All Rights Reserved.
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

package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"firebase.google.com/go/internal"
)

const providerConfigEndpoint = "https://identitytoolkit.googleapis.com/v2beta1"

// SAMLProviderConfig is the SAML auth provider configuration.
// See http://docs.oasis-open.org/security/saml/Post2.0/sstc-saml-tech-overview-2.0.html.
type SAMLProviderConfig struct {
	ID                    string
	DisplayName           string
	Enabled               bool
	IDPEntityID           string
	SSOURL                string
	RequestSigningEnabled bool
	X509Certificates      []string
	RPEntityID            string
	CallbackURL           string
}

type providerConfigClient struct {
	endpoint   string
	projectID  string
	httpClient *internal.HTTPClient
}

func newProviderConfigClient(hc *http.Client, conf *internal.AuthConfig) *providerConfigClient {
	client := &internal.HTTPClient{
		Client:      hc,
		SuccessFn:   internal.HasSuccessStatus,
		CreateErrFn: handleHTTPError,
		Opts: []internal.HTTPOption{
			internal.WithHeader("X-Client-Version", fmt.Sprintf("Go/Admin/%s", conf.Version)),
		},
	}
	return &providerConfigClient{
		endpoint:   providerConfigEndpoint,
		projectID:  conf.ProjectID,
		httpClient: client,
	}
}

// SAMLProviderConfig returns the SAMLProviderConfig with the given ID.
func (c *providerConfigClient) SAMLProviderConfig(ctx context.Context, id string) (*SAMLProviderConfig, error) {
	if err := validateSAMLConfigID(id); err != nil {
		return nil, err
	}

	req := &internal.Request{
		Method: http.MethodGet,
		URL:    fmt.Sprintf("/inboundSamlConfigs/%s", id),
	}
	var result samlProviderConfigDAO
	if _, err := c.makeRequest(ctx, req, &result); err != nil {
		return nil, err
	}

	return result.toSAMLProviderConfig(), nil
}

// DeleteSAMLProviderConfig deletes the SAMLProviderConfig with the given ID.
func (c *providerConfigClient) DeleteSAMLProviderConfig(ctx context.Context, id string) error {
	if err := validateSAMLConfigID(id); err != nil {
		return err
	}

	req := &internal.Request{
		Method: http.MethodDelete,
		URL:    fmt.Sprintf("/inboundSamlConfigs/%s", id),
	}
	_, err := c.makeRequest(ctx, req, nil)
	return err
}

func (c *providerConfigClient) makeRequest(ctx context.Context, req *internal.Request, v interface{}) (*internal.Response, error) {
	if c.projectID == "" {
		return nil, errors.New("project id not available")
	}

	req.URL = fmt.Sprintf("%s/projects/%s%s", c.endpoint, c.projectID, req.URL)
	return c.httpClient.DoAndUnmarshal(ctx, req, v)
}

type samlProviderConfigDAO struct {
	Name      string `json:"name"`
	IDPConfig struct {
		IDPEntityID     string `json:"idpEntityId"`
		SSOURL          string `json:"ssoUrl"`
		IDPCertificates []struct {
			X509Certificate string `json:"x509Certificate"`
		} `json:"idpCertificates"`
		SignRequest bool `json:"signRequest"`
	} `json:"idpConfig"`
	SPConfig struct {
		SPEntityID  string `json:"spEntityId"`
		CallbackURI string `json:"callbackUri"`
	} `json:"spConfig"`
	DisplayName string `json:"displayName"`
	Enabled     bool   `json:"enabled"`
}

func (dao *samlProviderConfigDAO) toSAMLProviderConfig() *SAMLProviderConfig {
	var certs []string
	for _, cert := range dao.IDPConfig.IDPCertificates {
		certs = append(certs, cert.X509Certificate)
	}

	return &SAMLProviderConfig{
		ID:                    extractResourceID(dao.Name),
		DisplayName:           dao.DisplayName,
		Enabled:               dao.Enabled,
		IDPEntityID:           dao.IDPConfig.IDPEntityID,
		SSOURL:                dao.IDPConfig.SSOURL,
		RequestSigningEnabled: dao.IDPConfig.SignRequest,
		X509Certificates:      certs,
		RPEntityID:            dao.SPConfig.SPEntityID,
		CallbackURL:           dao.SPConfig.CallbackURI,
	}
}

func validateSAMLConfigID(id string) error {
	if !strings.HasPrefix(id, "saml.") {
		return fmt.Errorf("invalid SAML provider id: %q", id)
	}

	return nil
}

func extractResourceID(name string) string {
	// name format: "projects/project-id/resource/resource-id"
	segments := strings.Split(name, "/")
	return segments[len(segments)-1]
}
