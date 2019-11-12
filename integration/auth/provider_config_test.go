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
	"fmt"
	"log"
	"reflect"
	"testing"

	"firebase.google.com/go/auth"
	"google.golang.org/api/iterator"
)

var x509Certs = []string{
	"-----BEGIN CERTIFICATE-----\nMIICZjCCAc+gAwIBAgIBADANBgkqhkiG9w0BAQ0FADBQMQswCQYDVQQGEwJ1czEL\nMAkGA1UECAwCQ0ExDTALBgNVBAoMBEFjbWUxETAPBgNVBAMMCGFjbWUuY29tMRIw\nEAYDVQQHDAlTdW5ueXZhbGUwHhcNMTgxMjA2MDc1MTUxWhcNMjgxMjAzMDc1MTUx\nWjBQMQswCQYDVQQGEwJ1czELMAkGA1UECAwCQ0ExDTALBgNVBAoMBEFjbWUxETAP\nBgNVBAMMCGFjbWUuY29tMRIwEAYDVQQHDAlTdW5ueXZhbGUwgZ8wDQYJKoZIhvcN\nAQEBBQADgY0AMIGJAoGBAKphmggjiVgqMLXyzvI7cKphscIIQ+wcv7Dld6MD4aKv\n7Jqr8ltujMxBUeY4LFEKw8Terb01snYpDotfilaG6NxpF/GfVVmMalzwWp0mT8+H\nyzyPj89mRcozu17RwuooR6n1ofXjGcBE86lqC21UhA3WVgjPOLqB42rlE9gPnZLB\nAgMBAAGjUDBOMB0GA1UdDgQWBBS0iM7WnbCNOnieOP1HIA+Oz/ML+zAfBgNVHSME\nGDAWgBS0iM7WnbCNOnieOP1HIA+Oz/ML+zAMBgNVHRMEBTADAQH/MA0GCSqGSIb3\nDQEBDQUAA4GBAF3jBgS+wP+K/jTupEQur6iaqS4UvXd//d4vo1MV06oTLQMTz+rP\nOSMDNwxzfaOn6vgYLKP/Dcy9dSTnSzgxLAxfKvDQZA0vE3udsw0Bd245MmX4+GOp\nlbrN99XP1u+lFxCSdMUzvQ/jW4ysw/Nq4JdJ0gPAyPvL6Qi/3mQdIQwx\n-----END CERTIFICATE-----\n",
	"-----BEGIN CERTIFICATE-----\nMIICZjCCAc+gAwIBAgIBADANBgkqhkiG9w0BAQ0FADBQMQswCQYDVQQGEwJ1czEL\nMAkGA1UECAwCQ0ExDTALBgNVBAoMBEFjbWUxETAPBgNVBAMMCGFjbWUuY29tMRIw\nEAYDVQQHDAlTdW5ueXZhbGUwHhcNMTgxMjA2MDc1ODE4WhcNMjgxMjAzMDc1ODE4\nWjBQMQswCQYDVQQGEwJ1czELMAkGA1UECAwCQ0ExDTALBgNVBAoMBEFjbWUxETAP\nBgNVBAMMCGFjbWUuY29tMRIwEAYDVQQHDAlTdW5ueXZhbGUwgZ8wDQYJKoZIhvcN\nAQEBBQADgY0AMIGJAoGBAKuzYKfDZGA6DJgQru3wNUqv+S0hMZfP/jbp8ou/8UKu\nrNeX7cfCgt3yxoGCJYKmF6t5mvo76JY0MWwA53BxeP/oyXmJ93uHG5mFRAsVAUKs\ncVVb0Xi6ujxZGVdDWFV696L0BNOoHTfXmac6IBoZQzNNK4n1AATqwo+z7a0pfRrJ\nAgMBAAGjUDBOMB0GA1UdDgQWBBSKmi/ZKMuLN0ES7/jPa7q7jAjPiDAfBgNVHSME\nGDAWgBSKmi/ZKMuLN0ES7/jPa7q7jAjPiDAMBgNVHRMEBTADAQH/MA0GCSqGSIb3\nDQEBDQUAA4GBAAg2a2kSn05NiUOuWOHwPUjW3wQRsGxPXtbhWMhmNdCfKKteM2+/\nLd/jz5F3qkOgGQ3UDgr3SHEoWhnLaJMF4a2tm6vL2rEIfPEK81KhTTRxSsAgMVbU\nJXBz1md6Ur0HlgQC7d1CHC8/xi2DDwHopLyxhogaZUxy9IaRxUEa2vJW\n-----END CERTIFICATE-----\n",
}

func TestOIDCProviderConfig(t *testing.T) {
	testOIDCProviderConfig(t, client)
}

type oidcProviderClient interface {
	OIDCProviderConfig(ctx context.Context, id string) (*auth.OIDCProviderConfig, error)
	OIDCProviderConfigs(ctx context.Context, nextPageToken string) *auth.OIDCProviderConfigIterator
	CreateOIDCProviderConfig(ctx context.Context, config *auth.OIDCProviderConfigToCreate) (*auth.OIDCProviderConfig, error)
	UpdateOIDCProviderConfig(ctx context.Context, id string, config *auth.OIDCProviderConfigToUpdate) (*auth.OIDCProviderConfig, error)
	DeleteOIDCProviderConfig(ctx context.Context, id string) error
}

func testOIDCProviderConfig(t *testing.T, client oidcProviderClient) {
	id := randomOIDCProviderID()
	want := &auth.OIDCProviderConfig{
		ID:          id,
		DisplayName: "OIDC_DISPLAY_NAME",
		Enabled:     true,
		ClientID:    "OIDC_CLIENT_ID",
		Issuer:      "https://oidc.com/issuer",
	}

	req := (&auth.OIDCProviderConfigToCreate{}).
		ID(id).
		DisplayName("OIDC_DISPLAY_NAME").
		Enabled(true).
		ClientID("OIDC_CLIENT_ID").
		Issuer("https://oidc.com/issuer")
	created, err := client.CreateOIDCProviderConfig(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateOIDCProviderConfig() = %v", err)
	}

	// Clean up action in the event of a panic
	defer func() {
		if id == "" {
			return
		}
		if err := client.DeleteOIDCProviderConfig(context.Background(), id); err != nil {
			log.Printf("WARN: failed to delete OIDC provider config %q on tear down: %v", id, err)
		}
	}()

	t.Run("CreateOIDCProviderConfig()", func(t *testing.T) {
		if !reflect.DeepEqual(created, want) {
			t.Errorf("CreateOIDCProviderConfig() = %#v; want = %#v", created, want)
		}
	})

	t.Run("OIDCProviderConfig()", func(t *testing.T) {
		oidc, err := client.OIDCProviderConfig(context.Background(), id)
		if err != nil {
			t.Fatalf("OIDCProviderConfig() = %v", err)
		}

		if !reflect.DeepEqual(oidc, want) {
			t.Errorf("OIDCProviderConfig() = %#v; want = %#v", oidc, want)
		}
	})

	t.Run("OIDCProviderConfigs()", func(t *testing.T) {
		iter := client.OIDCProviderConfigs(context.Background(), "")
		var target *auth.OIDCProviderConfig
		for {
			oidc, err := iter.Next()
			if err == iterator.Done {
				break
			} else if err != nil {
				t.Fatalf("OIDCProviderConfigs() = %v", err)
			}

			if oidc.ID == id {
				target = oidc
				break
			}
		}

		if target == nil {
			t.Fatalf("OIDCProviderConfigs() did not return required config: %q", id)
		}
		if !reflect.DeepEqual(target, want) {
			t.Errorf("OIDCProviderConfigs() = %#v; want = %#v", target, want)
		}
	})

	t.Run("UpdateOIDCProviderConfig()", func(t *testing.T) {
		want = &auth.OIDCProviderConfig{
			ID:          id,
			DisplayName: "UPDATED_OIDC_DISPLAY_NAME",
			ClientID:    "UPDATED_OIDC_CLIENT_ID",
			Issuer:      "https://oidc.com/updated_issuer",
		}
		req := (&auth.OIDCProviderConfigToUpdate{}).
			DisplayName("UPDATED_OIDC_DISPLAY_NAME").
			Enabled(false).
			ClientID("UPDATED_OIDC_CLIENT_ID").
			Issuer("https://oidc.com/updated_issuer")
		oidc, err := client.UpdateOIDCProviderConfig(context.Background(), id, req)
		if err != nil {
			t.Fatalf("UpdateOIDCProviderConfig() = %v", err)
		}

		if !reflect.DeepEqual(oidc, want) {
			t.Errorf("UpdateOIDCProviderConfig() = %#v; want = %#v", oidc, want)
		}
	})

	t.Run("DeleteOIDCProviderConfig()", func(t *testing.T) {
		if err := client.DeleteOIDCProviderConfig(context.Background(), id); err != nil {
			t.Fatalf("DeleteOIDCProviderConfig() = %v", err)
		}

		_, err := client.OIDCProviderConfig(context.Background(), id)
		if err == nil || !auth.IsConfigurationNotFound(err) {
			t.Errorf("OIDCProviderConfig() = %v; want = ConfigurationNotFound", err)
		}

		id = ""
	})
}

func TestSAMLProviderConfig(t *testing.T) {
	testSAMLProviderConfig(t, client)
}

type samlProviderClient interface {
	SAMLProviderConfig(ctx context.Context, id string) (*auth.SAMLProviderConfig, error)
	SAMLProviderConfigs(ctx context.Context, nextPageToken string) *auth.SAMLProviderConfigIterator
	CreateSAMLProviderConfig(ctx context.Context, config *auth.SAMLProviderConfigToCreate) (*auth.SAMLProviderConfig, error)
	UpdateSAMLProviderConfig(ctx context.Context, id string, config *auth.SAMLProviderConfigToUpdate) (*auth.SAMLProviderConfig, error)
	DeleteSAMLProviderConfig(ctx context.Context, id string) error
}

func testSAMLProviderConfig(t *testing.T, client samlProviderClient) {
	id := randomSAMLProviderID()
	want := &auth.SAMLProviderConfig{
		ID:          id,
		DisplayName: "SAML_DISPLAY_NAME",
		Enabled:     true,
		IDPEntityID: "IDP_ENTITY_ID",
		SSOURL:      "https://example.com/login",
		X509Certificates: []string{
			x509Certs[0],
		},
		RPEntityID:            "RP_ENTITY_ID",
		CallbackURL:           "https://projectId.firebaseapp.com/__/auth/handler",
		RequestSigningEnabled: true,
	}

	req := (&auth.SAMLProviderConfigToCreate{}).
		ID(id).
		DisplayName("SAML_DISPLAY_NAME").
		Enabled(true).
		IDPEntityID("IDP_ENTITY_ID").
		SSOURL("https://example.com/login").
		X509Certificates([]string{x509Certs[0]}).
		RPEntityID("RP_ENTITY_ID").
		CallbackURL("https://projectId.firebaseapp.com/__/auth/handler").
		RequestSigningEnabled(true)
	created, err := client.CreateSAMLProviderConfig(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateSAMLProviderConfig() = %v", err)
	}

	// Clean up action in the event of a panic
	defer func() {
		if id == "" {
			return
		}
		if err := client.DeleteSAMLProviderConfig(context.Background(), id); err != nil {
			log.Printf("WARN: failed to delete SAML provider config %q on tear down: %v", id, err)
		}
	}()

	t.Run("CreateSAMLProviderConfig()", func(t *testing.T) {
		if !reflect.DeepEqual(created, want) {
			t.Errorf("CreateSAMLProviderConfig() = %#v; want = %#v", created, want)
		}
	})

	t.Run("SAMLProviderConfig()", func(t *testing.T) {
		saml, err := client.SAMLProviderConfig(context.Background(), id)
		if err != nil {
			t.Fatalf("SAMLProviderConfig() = %v", err)
		}

		if !reflect.DeepEqual(saml, want) {
			t.Errorf("SAMLProviderConfig() = %#v; want = %#v", saml, want)
		}
	})

	t.Run("SAMLProviderConfigs()", func(t *testing.T) {
		iter := client.SAMLProviderConfigs(context.Background(), "")
		var target *auth.SAMLProviderConfig
		for {
			saml, err := iter.Next()
			if err == iterator.Done {
				break
			} else if err != nil {
				t.Fatalf("SAMLProviderConfigs() = %v", err)
			}

			if saml.ID == id {
				target = saml
				break
			}
		}

		if target == nil {
			t.Fatalf("SAMLProviderConfigs() did not return required config: %q", id)
		}
		if !reflect.DeepEqual(target, want) {
			t.Errorf("SAMLProviderConfigs() = %#v; want = %#v", target, want)
		}
	})

	t.Run("UpdateSAMLProviderConfig()", func(t *testing.T) {
		want = &auth.SAMLProviderConfig{
			ID:          id,
			DisplayName: "UPDATED_SAML_DISPLAY_NAME",
			IDPEntityID: "UPDATED_IDP_ENTITY_ID",
			SSOURL:      "https://example.com/updated_login",
			X509Certificates: []string{
				x509Certs[1],
			},
			RPEntityID:  "UPDATED_RP_ENTITY_ID",
			CallbackURL: "https://updatedProjectId.firebaseapp.com/__/auth/handler",
		}
		req := (&auth.SAMLProviderConfigToUpdate{}).
			DisplayName("UPDATED_SAML_DISPLAY_NAME").
			Enabled(false).
			IDPEntityID("UPDATED_IDP_ENTITY_ID").
			SSOURL("https://example.com/updated_login").
			X509Certificates([]string{x509Certs[1]}).
			RPEntityID("UPDATED_RP_ENTITY_ID").
			CallbackURL("https://updatedProjectId.firebaseapp.com/__/auth/handler").
			RequestSigningEnabled(false)
		saml, err := client.UpdateSAMLProviderConfig(context.Background(), id, req)
		if err != nil {
			t.Fatalf("UpdateSAMLProviderConfig() = %v", err)
		}

		if !reflect.DeepEqual(saml, want) {
			t.Errorf("UpdateSAMLProviderConfig() = %#v; want = %#v", saml, want)
		}
	})

	t.Run("DeleteSAMLProviderConfig()", func(t *testing.T) {
		if err := client.DeleteSAMLProviderConfig(context.Background(), id); err != nil {
			t.Fatalf("DeleteSAMLProviderConfig() = %v", err)
		}

		_, err := client.SAMLProviderConfig(context.Background(), id)
		if err == nil || !auth.IsConfigurationNotFound(err) {
			t.Errorf("SAMLProviderConfig() = %v; want = ConfigurationNotFound", err)
		}

		id = ""
	})
}

func randomSAMLProviderID() string {
	return fmt.Sprintf("saml.%s", randomCharacterString())
}

func randomOIDCProviderID() string {
	return fmt.Sprintf("oidc.%s", randomCharacterString())
}

func randomCharacterString() string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyz")
	b := make([]rune, 10)
	for i := range b {
		b[i] = letters[seededRand.Intn(len(letters))]
	}
	return string(b)
}

func deleteSAMLProviderConfig(id string) {
	if err := client.DeleteSAMLProviderConfig(context.Background(), id); err != nil {
		log.Printf("WARN: failed to delete SAML provider config %q on tear down: %v", id, err)
	}
}
