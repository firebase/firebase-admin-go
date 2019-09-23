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
	"net/http"
	"reflect"
	"strings"
	"testing"
)

const samlConfigResponse = `{
    "name":"projects/mock-project-id/inboundSamlConfigs/saml.provider",
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
	invalidIDs := []string{
		"",
		"invalid.id",
		"oidc.config",
	}
	wantErr := "invalid SAML provider id: "

	for _, id := range invalidIDs {
		saml, err := client.SAMLProviderConfig(context.Background(), id)
		if saml != nil || err == nil || !strings.HasPrefix(err.Error(), wantErr) {
			t.Errorf("SAMLProviderConfig(%q) = (%v, %v); want = (nil, %q)", id, saml, err, wantErr)
		}
	}
}

func TestSAMLProviderConfigNoProjectID(t *testing.T) {
	client := &providerConfigClient{}
	want := "project id not available"
	if _, err := client.SAMLProviderConfig(context.Background(), "saml.provider"); err == nil || err.Error() != want {
		t.Errorf("SAMLProviderConfig() = %v; want = %q", err, want)
	}
}
