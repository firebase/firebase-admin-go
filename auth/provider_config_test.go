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
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"

	"google.golang.org/api/iterator"
)

const oidcConfigResponse = `{
    "name":"projects/mock-project-id/oauthIdpConfigs/oidc.provider",
    "clientId": "CLIENT_ID",
    "issuer": "https://oidc.com/issuer",
    "displayName": "oidcProviderName",
    "enabled": true
}`

const samlConfigResponse = `{
    "name": "projects/mock-project-id/inboundSamlConfigs/saml.provider",
    "idpConfig": {
        "idpEntityId": "IDP_ENTITY_ID",
        "ssoUrl": "https://example.com/login",
        "signRequest": true,
        "idpCertificates": [
            {"x509Certificate": "CERT1"},
            {"x509Certificate": "CERT2"}
        ]
    },
    "spConfig": {
        "spEntityId": "RP_ENTITY_ID",
        "callbackUri": "https://projectId.firebaseapp.com/__/auth/handler"
    },
    "displayName": "samlProviderName",
    "enabled": true
}`

const notFoundResponse = `{
	"error": {
		"message": "CONFIGURATION_NOT_FOUND"
	}
}`

var idpCertsMap = []interface{}{
	map[string]interface{}{"x509Certificate": "CERT1"},
	map[string]interface{}{"x509Certificate": "CERT2"},
}

var oidcProviderConfig = &OIDCProviderConfig{
	ID:          "oidc.provider",
	DisplayName: "oidcProviderName",
	Enabled:     true,
	ClientID:    "CLIENT_ID",
	Issuer:      "https://oidc.com/issuer",
}

var samlProviderConfig = &SAMLProviderConfig{
	ID:                    "saml.provider",
	DisplayName:           "samlProviderName",
	Enabled:               true,
	IDPEntityID:           "IDP_ENTITY_ID",
	SSOURL:                "https://example.com/login",
	RequestSigningEnabled: true,
	X509Certificates:      []string{"CERT1", "CERT2"},
	RPEntityID:            "RP_ENTITY_ID",
	CallbackURL:           "https://projectId.firebaseapp.com/__/auth/handler",
}

var invalidOIDCConfigIDs = []string{
	"",
	"invalid.id",
	"saml.config",
}

var invalidSAMLConfigIDs = []string{
	"",
	"invalid.id",
	"oidc.config",
}

func TestOIDCProviderConfig(t *testing.T) {
	s := echoServer([]byte(oidcConfigResponse), t)
	defer s.Close()

	client := s.Client.pcc
	oidc, err := client.OIDCProviderConfig(context.Background(), "oidc.provider")
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(oidc, oidcProviderConfig) {
		t.Errorf("OIDCProviderConfig() = %#v; want = %#v", oidc, oidcProviderConfig)
	}

	req := s.Req[0]
	if req.Method != http.MethodGet {
		t.Errorf("OIDCProviderConfig() Method = %q; want = %q", req.Method, http.MethodGet)
	}

	wantURL := "/projects/mock-project-id/oauthIdpConfigs/oidc.provider"
	if req.URL.Path != wantURL {
		t.Errorf("OIDCProviderConfig() URL = %q; want = %q", req.URL.Path, wantURL)
	}
}

func TestOIDCProviderConfigInvalidID(t *testing.T) {
	client := &providerConfigClient{}
	wantErr := "invalid OIDC provider id: "

	for _, id := range invalidOIDCConfigIDs {
		saml, err := client.OIDCProviderConfig(context.Background(), id)
		if saml != nil || err == nil || !strings.HasPrefix(err.Error(), wantErr) {
			t.Errorf("OIDCProviderConfig(%q) = (%v, %v); want = (nil, %q)", id, saml, err, wantErr)
		}
	}
}

func TestOIDCProviderConfigError(t *testing.T) {
	s := echoServer([]byte(notFoundResponse), t)
	defer s.Close()
	s.Status = http.StatusNotFound

	client := s.Client.pcc
	saml, err := client.OIDCProviderConfig(context.Background(), "oidc.provider")
	if saml != nil || err == nil || !IsConfigurationNotFound(err) {
		t.Errorf("OIDCProviderConfig() = (%v, %v); want = (nil, ConfigurationNotFound)", saml, err)
	}
}

func TestDeleteOIDCProviderConfig(t *testing.T) {
	s := echoServer([]byte("{}"), t)
	defer s.Close()

	client := s.Client.pcc
	if err := client.DeleteOIDCProviderConfig(context.Background(), "oidc.provider"); err != nil {
		t.Fatal(err)
	}

	req := s.Req[0]
	if req.Method != http.MethodDelete {
		t.Errorf("DeleteOIDCProviderConfig() Method = %q; want = %q", req.Method, http.MethodDelete)
	}

	wantURL := "/projects/mock-project-id/oauthIdpConfigs/oidc.provider"
	if req.URL.Path != wantURL {
		t.Errorf("DeleteOIDCProviderConfig() URL = %q; want = %q", req.URL.Path, wantURL)
	}
}

func TestDeleteOIDCProviderConfigInvalidID(t *testing.T) {
	client := &providerConfigClient{}
	wantErr := "invalid OIDC provider id: "

	for _, id := range invalidOIDCConfigIDs {
		err := client.DeleteOIDCProviderConfig(context.Background(), id)
		if err == nil || !strings.HasPrefix(err.Error(), wantErr) {
			t.Errorf("DeleteOIDCProviderConfig(%q) = %v; want = %q", id, err, wantErr)
		}
	}
}

func TestDeleteOIDCProviderConfigError(t *testing.T) {
	s := echoServer([]byte(notFoundResponse), t)
	defer s.Close()
	s.Status = http.StatusNotFound

	client := s.Client.pcc
	err := client.DeleteOIDCProviderConfig(context.Background(), "oidc.provider")
	if err == nil || !IsConfigurationNotFound(err) {
		t.Errorf("DeleteOIDCProviderConfig() = %v; want = ConfigurationNotFound", err)
	}
}

func TestSAMLProviderConfig(t *testing.T) {
	s := echoServer([]byte(samlConfigResponse), t)
	defer s.Close()

	client := s.Client.pcc
	saml, err := client.SAMLProviderConfig(context.Background(), "saml.provider")
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(saml, samlProviderConfig) {
		t.Errorf("SAMLProviderConfig() = %#v; want = %#v", saml, samlProviderConfig)
	}

	req := s.Req[0]
	if req.Method != http.MethodGet {
		t.Errorf("SAMLProviderConfig() Method = %q; want = %q", req.Method, http.MethodGet)
	}

	wantURL := "/projects/mock-project-id/inboundSamlConfigs/saml.provider"
	if req.URL.Path != wantURL {
		t.Errorf("SAMLProviderConfig() URL = %q; want = %q", req.URL.Path, wantURL)
	}
}

func TestSAMLProviderConfigInvalidID(t *testing.T) {
	client := &providerConfigClient{}
	wantErr := "invalid SAML provider id: "

	for _, id := range invalidSAMLConfigIDs {
		saml, err := client.SAMLProviderConfig(context.Background(), id)
		if saml != nil || err == nil || !strings.HasPrefix(err.Error(), wantErr) {
			t.Errorf("SAMLProviderConfig(%q) = (%v, %v); want = (nil, %q)", id, saml, err, wantErr)
		}
	}
}

func TestSAMLProviderConfigError(t *testing.T) {
	s := echoServer([]byte(notFoundResponse), t)
	defer s.Close()
	s.Status = http.StatusNotFound

	client := s.Client.pcc
	saml, err := client.SAMLProviderConfig(context.Background(), "saml.provider")
	if saml != nil || err == nil || !IsConfigurationNotFound(err) {
		t.Errorf("SAMLProviderConfig() = (%v, %v); want = (nil, ConfigurationNotFound)", saml, err)
	}
}

func TestCreateSAMLProviderConfig(t *testing.T) {
	s := echoServer([]byte(samlConfigResponse), t)
	defer s.Close()

	client := s.Client.pcc
	options := (&SAMLProviderConfigToCreate{}).
		ID(samlProviderConfig.ID).
		DisplayName(samlProviderConfig.DisplayName).
		Enabled(samlProviderConfig.Enabled).
		IDPEntityID(samlProviderConfig.IDPEntityID).
		SSOURL(samlProviderConfig.SSOURL).
		RequestSigningEnabled(samlProviderConfig.RequestSigningEnabled).
		X509Certificates(samlProviderConfig.X509Certificates).
		RPEntityID(samlProviderConfig.RPEntityID).
		CallbackURL(samlProviderConfig.CallbackURL)
	saml, err := client.CreateSAMLProviderConfig(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(saml, samlProviderConfig) {
		t.Errorf("CreateSAMLProviderConfig() = %#v; want = %#v", saml, samlProviderConfig)
	}

	wantBody := map[string]interface{}{
		"displayName": samlProviderConfig.DisplayName,
		"enabled":     samlProviderConfig.Enabled,
		"idpConfig": map[string]interface{}{
			"idpEntityId":     samlProviderConfig.IDPEntityID,
			"ssoUrl":          samlProviderConfig.SSOURL,
			"signRequest":     samlProviderConfig.RequestSigningEnabled,
			"idpCertificates": idpCertsMap,
		},
		"spConfig": map[string]interface{}{
			"spEntityId":  samlProviderConfig.RPEntityID,
			"callbackUri": samlProviderConfig.CallbackURL,
		},
	}
	if err := checkCreateRequest(s, wantBody); err != nil {
		t.Fatal(err)
	}
}

func TestCreateSAMLProviderConfigMinimal(t *testing.T) {
	s := echoServer([]byte(samlConfigResponse), t)
	defer s.Close()

	client := s.Client.pcc
	options := (&SAMLProviderConfigToCreate{}).
		ID(samlProviderConfig.ID).
		IDPEntityID(samlProviderConfig.IDPEntityID).
		SSOURL(samlProviderConfig.SSOURL).
		X509Certificates(samlProviderConfig.X509Certificates).
		RPEntityID(samlProviderConfig.RPEntityID).
		CallbackURL(samlProviderConfig.CallbackURL)
	saml, err := client.CreateSAMLProviderConfig(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(saml, samlProviderConfig) {
		t.Errorf("CreateSAMLProviderConfig() = %#v; want = %#v", saml, samlProviderConfig)
	}

	wantBody := map[string]interface{}{
		"idpConfig": map[string]interface{}{
			"idpEntityId":     samlProviderConfig.IDPEntityID,
			"ssoUrl":          samlProviderConfig.SSOURL,
			"idpCertificates": idpCertsMap,
		},
		"spConfig": map[string]interface{}{
			"spEntityId":  samlProviderConfig.RPEntityID,
			"callbackUri": samlProviderConfig.CallbackURL,
		},
	}
	if err := checkCreateRequest(s, wantBody); err != nil {
		t.Fatal(err)
	}
}

func TestCreateSAMLProviderConfigZeroValues(t *testing.T) {
	s := echoServer([]byte(samlConfigResponse), t)
	defer s.Close()
	client := s.Client.pcc

	options := (&SAMLProviderConfigToCreate{}).
		ID(samlProviderConfig.ID).
		DisplayName(samlProviderConfig.DisplayName).
		Enabled(false).
		IDPEntityID(samlProviderConfig.IDPEntityID).
		SSOURL(samlProviderConfig.SSOURL).
		RequestSigningEnabled(false).
		X509Certificates(samlProviderConfig.X509Certificates).
		RPEntityID(samlProviderConfig.RPEntityID).
		CallbackURL(samlProviderConfig.CallbackURL)
	saml, err := client.CreateSAMLProviderConfig(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(saml, samlProviderConfig) {
		t.Errorf("CreateSAMLProviderConfig() = %#v; want = %#v", saml, samlProviderConfig)
	}

	wantBody := map[string]interface{}{
		"displayName": samlProviderConfig.DisplayName,
		"enabled":     false,
		"idpConfig": map[string]interface{}{
			"idpEntityId":     samlProviderConfig.IDPEntityID,
			"ssoUrl":          samlProviderConfig.SSOURL,
			"signRequest":     false,
			"idpCertificates": idpCertsMap,
		},
		"spConfig": map[string]interface{}{
			"spEntityId":  samlProviderConfig.RPEntityID,
			"callbackUri": samlProviderConfig.CallbackURL,
		},
	}
	if err := checkCreateRequest(s, wantBody); err != nil {
		t.Fatal(err)
	}
}

func TestCreateSAMLProviderConfigError(t *testing.T) {
	s := echoServer([]byte("{}"), t)
	s.Status = http.StatusInternalServerError
	defer s.Close()

	client := s.Client.pcc
	options := (&SAMLProviderConfigToCreate{}).
		ID(samlProviderConfig.ID).
		DisplayName(samlProviderConfig.DisplayName).
		Enabled(samlProviderConfig.Enabled).
		IDPEntityID(samlProviderConfig.IDPEntityID).
		SSOURL(samlProviderConfig.SSOURL).
		RequestSigningEnabled(samlProviderConfig.RequestSigningEnabled).
		X509Certificates(samlProviderConfig.X509Certificates).
		RPEntityID(samlProviderConfig.RPEntityID).
		CallbackURL(samlProviderConfig.CallbackURL)
	saml, err := client.CreateSAMLProviderConfig(context.Background(), options)
	if saml != nil || !IsUnknown(err) {
		t.Errorf("SAMLProviderConfig() = (%v, %v); want = (nil, %q)", saml, err, "unknown-error")
	}
}

func TestCreateSAMLProviderConfigInvalidInput(t *testing.T) {
	cases := []struct {
		name string
		want string
		conf *SAMLProviderConfigToCreate
	}{
		{
			name: "NilConfig",
			want: "config must not be nil",
			conf: nil,
		},
		{
			name: "EmptyID",
			want: "invalid SAML provider id: ",
			conf: &SAMLProviderConfigToCreate{},
		},
		{
			name: "InvalidID",
			want: "invalid SAML provider id: ",
			conf: (&SAMLProviderConfigToCreate{}).
				ID("oidc.provider"),
		},
		{
			name: "EmptyOptions",
			want: "no parameters specified in the create request",
			conf: (&SAMLProviderConfigToCreate{}).
				ID("saml.provider"),
		},
		{
			name: "EmptyIDPEntityID",
			want: "IDPEntityID must not be empty",
			conf: (&SAMLProviderConfigToCreate{}).
				ID("saml.provider").
				IDPEntityID(""),
		},
		{
			name: "EmptySSOURL",
			want: "SSOURL must not be empty",
			conf: (&SAMLProviderConfigToCreate{}).
				ID("saml.provider").
				IDPEntityID("IDP_ENTITY_ID"),
		},
		{
			name: "InvalidSSOURL",
			want: "failed to parse SSOURL: ",
			conf: (&SAMLProviderConfigToCreate{}).
				ID("saml.provider").
				IDPEntityID("IDP_ENTITY_ID").
				SSOURL("not a url"),
		},
		{
			name: "EmptyX509Certs",
			want: "X509Certificates must not be empty",
			conf: (&SAMLProviderConfigToCreate{}).
				ID("saml.provider").
				IDPEntityID("IDP_ENTITY_ID").
				SSOURL("https://example.com/login"),
		},
		{
			name: "EmptyStringInX509Certs",
			want: "X509Certificates must not contain empty strings",
			conf: (&SAMLProviderConfigToCreate{}).
				ID("saml.provider").
				IDPEntityID("IDP_ENTITY_ID").
				SSOURL("https://example.com/login").
				X509Certificates([]string{""}),
		},
		{
			name: "EmptyRPEntityID",
			want: "RPEntityID must not be empty",
			conf: (&SAMLProviderConfigToCreate{}).
				ID("saml.provider").
				IDPEntityID("IDP_ENTITY_ID").
				SSOURL("https://example.com/login").
				X509Certificates([]string{"CERT"}),
		},
		{
			name: "EmptyCallbackURL",
			want: "CallbackURL must not be empty",
			conf: (&SAMLProviderConfigToCreate{}).
				ID("saml.provider").
				IDPEntityID("IDP_ENTITY_ID").
				SSOURL("https://example.com/login").
				X509Certificates([]string{"CERT"}).
				RPEntityID("RP_ENTITY_ID"),
		},
		{
			name: "InvalidCallbackURL",
			want: "failed to parse CallbackURL: ",
			conf: (&SAMLProviderConfigToCreate{}).
				ID("saml.provider").
				IDPEntityID("IDP_ENTITY_ID").
				SSOURL("https://example.com/login").
				X509Certificates([]string{"CERT"}).
				RPEntityID("RP_ENTITY_ID").
				CallbackURL("not a url"),
		},
	}

	client := &providerConfigClient{}
	for _, tc := range cases {
		_, err := client.CreateSAMLProviderConfig(context.Background(), tc.conf)
		if err == nil || !strings.HasPrefix(err.Error(), tc.want) {
			t.Errorf("CreateSAMLProviderConfig(%q) = %v; want = %q", tc.name, err, tc.want)
		}
	}
}

func TestUpdateSAMLProviderConfig(t *testing.T) {
	s := echoServer([]byte(samlConfigResponse), t)
	defer s.Close()

	client := s.Client.pcc
	options := (&SAMLProviderConfigToUpdate{}).
		DisplayName(samlProviderConfig.DisplayName).
		Enabled(samlProviderConfig.Enabled).
		IDPEntityID(samlProviderConfig.IDPEntityID).
		SSOURL(samlProviderConfig.SSOURL).
		RequestSigningEnabled(samlProviderConfig.RequestSigningEnabled).
		X509Certificates(samlProviderConfig.X509Certificates).
		RPEntityID(samlProviderConfig.RPEntityID).
		CallbackURL(samlProviderConfig.CallbackURL)
	saml, err := client.UpdateSAMLProviderConfig(context.Background(), "saml.provider", options)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(saml, samlProviderConfig) {
		t.Errorf("UpdateSAMLProviderConfig() = %#v; want = %#v", saml, samlProviderConfig)
	}

	wantBody := map[string]interface{}{
		"displayName": samlProviderConfig.DisplayName,
		"enabled":     samlProviderConfig.Enabled,
		"idpConfig": map[string]interface{}{
			"idpEntityId":     samlProviderConfig.IDPEntityID,
			"ssoUrl":          samlProviderConfig.SSOURL,
			"signRequest":     samlProviderConfig.RequestSigningEnabled,
			"idpCertificates": idpCertsMap,
		},
		"spConfig": map[string]interface{}{
			"spEntityId":  samlProviderConfig.RPEntityID,
			"callbackUri": samlProviderConfig.CallbackURL,
		},
	}
	wantMask := []string{
		"displayName",
		"enabled",
		"idpConfig.idpCertificates",
		"idpConfig.idpEntityId",
		"idpConfig.signRequest",
		"idpConfig.ssoUrl",
		"spConfig.callbackUri",
		"spConfig.spEntityId",
	}
	if err := checkUpdateRequest(s, wantBody, wantMask); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateSAMLProviderConfigMinimal(t *testing.T) {
	s := echoServer([]byte(samlConfigResponse), t)
	defer s.Close()

	client := s.Client.pcc
	options := (&SAMLProviderConfigToUpdate{}).
		DisplayName("Other name")
	saml, err := client.UpdateSAMLProviderConfig(context.Background(), "saml.provider", options)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(saml, samlProviderConfig) {
		t.Errorf("UpdateSAMLProviderConfig() = %#v; want = %#v", saml, samlProviderConfig)
	}

	wantBody := map[string]interface{}{
		"displayName": "Other name",
	}
	wantMask := []string{
		"displayName",
	}
	if err := checkUpdateRequest(s, wantBody, wantMask); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateSAMLProviderConfigZeroValues(t *testing.T) {
	s := echoServer([]byte(samlConfigResponse), t)
	defer s.Close()

	client := s.Client.pcc
	options := (&SAMLProviderConfigToUpdate{}).
		DisplayName("").
		Enabled(false).
		RequestSigningEnabled(false)
	saml, err := client.UpdateSAMLProviderConfig(context.Background(), "saml.provider", options)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(saml, samlProviderConfig) {
		t.Errorf("UpdateSAMLProviderConfig() = %#v; want = %#v", saml, samlProviderConfig)
	}

	wantBody := map[string]interface{}{
		"displayName": nil,
		"enabled":     false,
		"idpConfig": map[string]interface{}{
			"signRequest": false,
		},
	}
	wantMask := []string{
		"displayName",
		"enabled",
		"idpConfig.signRequest",
	}
	if err := checkUpdateRequest(s, wantBody, wantMask); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateSAMLProviderConfigInvalidID(t *testing.T) {
	cases := []string{"", "oidc.config"}
	client := &providerConfigClient{}
	options := (&SAMLProviderConfigToUpdate{}).
		DisplayName("").
		Enabled(false).
		RequestSigningEnabled(false)
	want := "invalid SAML provider id: "
	for _, tc := range cases {
		_, err := client.UpdateSAMLProviderConfig(context.Background(), tc, options)
		if err == nil || !strings.HasPrefix(err.Error(), want) {
			t.Errorf("UpdateSAMLProviderConfig(%q) = %v; want = %q", tc, err, "foo")
		}
	}
}

func TestUpdateSAMLProviderConfigInvalidInput(t *testing.T) {
	cases := []struct {
		name string
		want string
		conf *SAMLProviderConfigToUpdate
	}{
		{
			name: "NilConfig",
			want: "config must not be nil",
			conf: nil,
		},
		{
			name: "Empty",
			want: "no parameters specified in the update request",
			conf: &SAMLProviderConfigToUpdate{},
		},
		{
			name: "EmptyIDPEntityID",
			want: "IDPEntityID must not be empty",
			conf: (&SAMLProviderConfigToUpdate{}).
				IDPEntityID(""),
		},
		{
			name: "EmptySSOURL",
			want: "SSOURL must not be empty",
			conf: (&SAMLProviderConfigToUpdate{}).
				SSOURL(""),
		},
		{
			name: "InvalidSSOURL",
			want: "failed to parse SSOURL: ",
			conf: (&SAMLProviderConfigToUpdate{}).
				SSOURL("not a url"),
		},
		{
			name: "EmptyX509Certs",
			want: "X509Certificates must not be empty",
			conf: (&SAMLProviderConfigToUpdate{}).
				X509Certificates(nil),
		},
		{
			name: "EmptyStringInX509Certs",
			want: "X509Certificates must not contain empty strings",
			conf: (&SAMLProviderConfigToUpdate{}).
				X509Certificates([]string{""}),
		},
		{
			name: "EmptyRPEntityID",
			want: "RPEntityID must not be empty",
			conf: (&SAMLProviderConfigToUpdate{}).
				RPEntityID(""),
		},
		{
			name: "EmptyCallbackURL",
			want: "CallbackURL must not be empty",
			conf: (&SAMLProviderConfigToUpdate{}).
				CallbackURL(""),
		},
		{
			name: "InvalidCallbackURL",
			want: "failed to parse CallbackURL: ",
			conf: (&SAMLProviderConfigToUpdate{}).
				CallbackURL("not a url"),
		},
	}

	client := &providerConfigClient{}
	for _, tc := range cases {
		_, err := client.UpdateSAMLProviderConfig(context.Background(), "saml.provider", tc.conf)
		if err == nil || !strings.HasPrefix(err.Error(), tc.want) {
			t.Errorf("UpdateSAMLProviderConfig(%q) = %v; want = %q", tc.name, err, tc.want)
		}
	}
}

func TestDeleteSAMLProviderConfig(t *testing.T) {
	s := echoServer([]byte("{}"), t)
	defer s.Close()

	client := s.Client.pcc
	if err := client.DeleteSAMLProviderConfig(context.Background(), "saml.provider"); err != nil {
		t.Fatal(err)
	}

	req := s.Req[0]
	if req.Method != http.MethodDelete {
		t.Errorf("DeleteSAMLProviderConfig() Method = %q; want = %q", req.Method, http.MethodDelete)
	}

	wantURL := "/projects/mock-project-id/inboundSamlConfigs/saml.provider"
	if req.URL.Path != wantURL {
		t.Errorf("DeleteSAMLProviderConfig() URL = %q; want = %q", req.URL.Path, wantURL)
	}
}

func TestDeleteSAMLProviderConfigInvalidID(t *testing.T) {
	client := &providerConfigClient{}
	wantErr := "invalid SAML provider id: "

	for _, id := range invalidSAMLConfigIDs {
		err := client.DeleteSAMLProviderConfig(context.Background(), id)
		if err == nil || !strings.HasPrefix(err.Error(), wantErr) {
			t.Errorf("DeleteSAMLProviderConfig(%q) = %v; want = %q", id, err, wantErr)
		}
	}
}

func TestDeleteSAMLProviderConfigError(t *testing.T) {
	s := echoServer([]byte(notFoundResponse), t)
	defer s.Close()
	s.Status = http.StatusNotFound

	client := s.Client.pcc
	err := client.DeleteSAMLProviderConfig(context.Background(), "saml.provider")
	if err == nil || !IsConfigurationNotFound(err) {
		t.Errorf("DeleteSAMLProviderConfig() = %v; want = ConfigurationNotFound", err)
	}
}

func TestSAMLProviderConfigs(t *testing.T) {
	template := `{
                "inboundSamlConfigs": [
                    %s,
                    %s,
                    %s
                ],
                "nextPageToken": ""
        }`
	response := fmt.Sprintf(template, samlConfigResponse, samlConfigResponse, samlConfigResponse)
	s := echoServer([]byte(response), t)
	defer s.Close()

	want := []*SAMLProviderConfig{
		samlProviderConfig,
		samlProviderConfig,
		samlProviderConfig,
	}

	testIterator := func(iter *SAMLProviderConfigIterator, token string, req string) {
		count := 0
		for i := 0; i < len(want); i++ {
			config, err := iter.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(config, want[i]) {
				t.Errorf("SAMLProviderConfigs(%q) = %#v; want = %#v", token, config, want[i])
			}
			count++
		}
		if count != len(want) {
			t.Errorf("SAMLProviderConfigs(%q) = %d; want = %d", token, count, len(want))
		}
		if _, err := iter.Next(); err != iterator.Done {
			t.Errorf("SAMLProviderConfigs(%q) = %v, want = %v", token, err, iterator.Done)
		}

		// Check the query string of the last HTTP request made.
		gotReq := s.Req[len(s.Req)-1].URL.Query().Encode()
		if gotReq != req {
			t.Errorf("SAMLProviderConfigs(%q) = %q, want = %v", token, gotReq, req)
		}
	}

	client := s.Client.pcc
	testIterator(
		client.SAMLProviderConfigs(context.Background(), ""),
		"",
		"pageSize=100")
	testIterator(
		client.SAMLProviderConfigs(context.Background(), "pageToken"),
		"pageToken",
		"pageSize=100&pageToken=pageToken")
}

func TestSAMLProviderConfigsError(t *testing.T) {
	s := echoServer([]byte("{}"), t)
	defer s.Close()
	s.Status = http.StatusInternalServerError

	client := s.Client.pcc
	it := client.SAMLProviderConfigs(context.Background(), "")
	config, err := it.Next()
	if config != nil || err == nil || !IsUnknown(err) {
		t.Errorf("SAMLProviderConfigs() = (%v, %v); want = (nil, %q)", config, err, "unknown-error")
	}
}

func TestSAMLProviderConfigNoProjectID(t *testing.T) {
	client := &providerConfigClient{}
	want := "project id not available"
	if _, err := client.SAMLProviderConfig(context.Background(), "saml.provider"); err == nil || err.Error() != want {
		t.Errorf("SAMLProviderConfig() = %v; want = %q", err, want)
	}
}

func checkCreateRequest(s *mockAuthServer, wantBody interface{}) error {
	req := s.Req[0]
	if req.Method != http.MethodPost {
		return fmt.Errorf("CreateSAMLProviderConfig() Method = %q; want = %q", req.Method, http.MethodPost)
	}

	wantURL := "/projects/mock-project-id/inboundSamlConfigs"
	if req.URL.Path != wantURL {
		return fmt.Errorf("CreateSAMLProviderConfig() URL = %q; want = %q", req.URL.Path, wantURL)
	}

	wantQuery := "inboundSamlConfigId=saml.provider"
	if req.URL.RawQuery != wantQuery {
		return fmt.Errorf("CreateSAMLProviderConfig() Query = %q; want = %q", req.URL.RawQuery, wantQuery)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(s.Rbody, &body); err != nil {
		return err
	}

	if !reflect.DeepEqual(body, wantBody) {
		return fmt.Errorf("CreateSAMLProviderConfig() Body = %#v; want = %#v", body, wantBody)
	}

	return nil
}

func checkUpdateRequest(s *mockAuthServer, wantBody interface{}, wantMask []string) error {
	req := s.Req[0]
	if req.Method != http.MethodPatch {
		return fmt.Errorf("UpdateSAMLProviderConfig() Method = %q; want = %q", req.Method, http.MethodPatch)
	}

	wantURL := "/projects/mock-project-id/inboundSamlConfigs/saml.provider"
	if req.URL.Path != wantURL {
		return fmt.Errorf("UpdateSAMLProviderConfig() URL = %q; want = %q", req.URL.Path, wantURL)
	}

	queryParam := req.URL.Query().Get("updateMask")
	mask := strings.Split(queryParam, ",")
	sort.Strings(mask)
	if !reflect.DeepEqual(mask, wantMask) {
		return fmt.Errorf("UpdateSAMLProviderConfig() Query = %#v; want = %#v", mask, wantMask)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(s.Rbody, &body); err != nil {
		return err
	}

	if !reflect.DeepEqual(body, wantBody) {
		return fmt.Errorf("UpdateSAMLProviderConfig() Body = %#v; want = %#v", body, wantBody)
	}

	return nil
}
