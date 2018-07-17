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

// Package auth contains functions for minting custom authentication tokens, and verifying Firebase ID tokens.
package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/net/context"

	"firebase.google.com/go/internal"
	"google.golang.org/api/identitytoolkit/v3"
	"google.golang.org/api/transport"
)

const (
	firebaseAudience = "https://identitytoolkit.googleapis.com/google.identity.identitytoolkit.v1.IdentityToolkit"
	idTokenCertURL   = "https://www.googleapis.com/robot/v1/metadata/x509/securetoken@system.gserviceaccount.com"
	issuerPrefix     = "https://securetoken.google.com/"
	tokenExpSeconds  = 3600
)

var reservedClaims = []string{
	"acr", "amr", "at_hash", "aud", "auth_time", "azp", "cnf", "c_hash",
	"exp", "firebase", "iat", "iss", "jti", "nbf", "nonce", "sub",
}

var clk clock = &systemClock{}

// Token represents a decoded Firebase ID token.
//
// Token provides typed accessors to the common JWT fields such as Audience (aud) and Expiry (exp).
// Additionally it provides a UID field, which indicates the user ID of the account to which this token
// belongs. Any additional JWT claims can be accessed via the Claims map of Token.
type Token struct {
	Issuer   string                 `json:"iss"`
	Audience string                 `json:"aud"`
	Expires  int64                  `json:"exp"`
	IssuedAt int64                  `json:"iat"`
	Subject  string                 `json:"sub,omitempty"`
	UID      string                 `json:"uid,omitempty"`
	Claims   map[string]interface{} `json:"-"`
}

// Client is the interface for the Firebase auth service.
//
// Client facilitates generating custom JWT tokens for Firebase clients, and verifying ID tokens issued
// by Firebase backend services.
type Client struct {
	is        *identitytoolkit.Service
	keySource keySource
	projectID string
	signer    cryptoSigner
	version   string
}

type signer interface {
	Email(ctx context.Context) (string, error)
	Sign(ctx context.Context, b []byte) ([]byte, error)
}

// NewClient creates a new instance of the Firebase Auth Client.
//
// This function can only be invoked from within the SDK. Client applications should access the
// Auth service through firebase.App.
func NewClient(ctx context.Context, conf *internal.AuthConfig) (*Client, error) {
	var (
		signer cryptoSigner
		err    error
	)
	// Initialize a signer by following the go/firebase-admin-sign protocol.
	if conf.Creds != nil && len(conf.Creds.JSON) > 0 {
		// If the SDK was initialized with a service account, use it to sign bytes.
		var sa serviceAccount
		if err = json.Unmarshal(conf.Creds.JSON, &sa); err != nil {
			return nil, err
		}
		if sa.PrivateKey != "" && sa.ClientEmail != "" {
			var err error
			signer, err = newServiceAccountSigner(sa)
			if err != nil {
				return nil, err
			}
		}
	}
	if signer == nil {
		if conf.ServiceAccountID != "" {
			// If the SDK was initialized with a service account email, use it with the IAM service
			// to sign bytes.
			signer, err = newIAMSigner(ctx, conf)
			if err != nil {
				return nil, err
			}
		} else {
			// Use GAE signing capabilities if available. Otherwise, obtain a service account email
			// from the local Metadata service, and fallback to the IAM service.
			signer, err = newCryptoSigner(ctx, conf)
			if err != nil {
				return nil, err
			}
		}
	}

	hc, _, err := transport.NewHTTPClient(ctx, conf.Opts...)
	if err != nil {
		return nil, err
	}

	is, err := identitytoolkit.New(hc)
	if err != nil {
		return nil, err
	}

	return &Client{
		is:        is,
		keySource: newHTTPKeySource(idTokenCertURL, http.DefaultClient),
		projectID: conf.ProjectID,
		signer:    signer,
		version:   "Go/Admin/" + conf.Version,
	}, nil
}

// CustomToken creates a signed custom authentication token with the specified user ID.
//
// The resulting JWT can be used in a Firebase client SDK to trigger an authentication flow. See
// https://firebase.google.com/docs/auth/admin/create-custom-tokens#sign_in_using_custom_tokens_on_clients
// for more details on how to use custom tokens for client authentication.
//
// CustomToken follows the protocol outlined below to sign the generated tokens:
//   - If the SDK was initialized with service account credentials, uses the private key present in
//     the credentials to sign tokens locally.
//   - If a service account email was specified during initialization (via firebase.Config struct),
//     calls the IAM service with that email to sign tokens remotely. See
//     https://cloud.google.com/iam/reference/rest/v1/projects.serviceAccounts/signBlob.
//   - If the code is deployed in the Google App Engine standard environment, uses the App Identity
//     service to sign tokens. See https://cloud.google.com/appengine/docs/standard/go/reference#SignBytes.
//   - If the code is deployed in a different GCP-managed environment (e.g. Google Compute Engine),
//     uses the local Metadata server to auto discover a service account email. This is used in
//     conjunction with the IAM service to sign tokens remotely.
//
// CustomToken returns an error the SDK fails to discover a viable mechanism for signing tokens.
func (c *Client) CustomToken(ctx context.Context, uid string) (string, error) {
	return c.CustomTokenWithClaims(ctx, uid, nil)
}

// CustomTokenWithClaims is similar to CustomToken, but in addition to the user ID, it also encodes
// all the key-value pairs in the provided map as claims in the resulting JWT.
func (c *Client) CustomTokenWithClaims(ctx context.Context, uid string, devClaims map[string]interface{}) (string, error) {
	iss, err := c.signer.Email(ctx)
	if err != nil {
		return "", err
	}

	if len(uid) == 0 || len(uid) > 128 {
		return "", errors.New("uid must be non-empty, and not longer than 128 characters")
	}

	var disallowed []string
	for _, k := range reservedClaims {
		if _, contains := devClaims[k]; contains {
			disallowed = append(disallowed, k)
		}
	}
	if len(disallowed) == 1 {
		return "", fmt.Errorf("developer claim %q is reserved and cannot be specified", disallowed[0])
	} else if len(disallowed) > 1 {
		return "", fmt.Errorf("developer claims %q are reserved and cannot be specified", strings.Join(disallowed, ", "))
	}

	now := clk.Now().Unix()
	info := &jwtInfo{
		header: jwtHeader{Algorithm: "RS256", Type: "JWT"},
		payload: &customToken{
			Iss:    iss,
			Sub:    iss,
			Aud:    firebaseAudience,
			UID:    uid,
			Iat:    now,
			Exp:    now + tokenExpSeconds,
			Claims: devClaims,
		},
	}
	return info.Token(ctx, c.signer)
}

// VerifyIDToken verifies the signature	and payload of the provided ID token.
//
// VerifyIDToken accepts a signed JWT token string, and verifies that it is current, issued for the
// correct Firebase project, and signed by the Google Firebase services in the cloud. It returns
// a Token containing the decoded claims in the input JWT. See
// https://firebase.google.com/docs/auth/admin/verify-id-tokens#retrieve_id_tokens_on_clients for
// more details on how to obtain an ID token in a client app.
// This does not check whether or not the token has been revoked. See `VerifyIDTokenAndCheckRevoked` below.
func (c *Client) VerifyIDToken(ctx context.Context, idToken string) (*Token, error) {
	if c.projectID == "" {
		return nil, errors.New("project id not available")
	}
	if idToken == "" {
		return nil, fmt.Errorf("id token must be a non-empty string")
	}

	if err := verifyToken(ctx, idToken, c.keySource); err != nil {
		return nil, err
	}
	segments := strings.Split(idToken, ".")

	var (
		header  jwtHeader
		payload Token
		claims  map[string]interface{}
	)
	if err := decode(segments[0], &header); err != nil {
		return nil, err
	}
	if err := decode(segments[1], &payload); err != nil {
		return nil, err
	}
	if err := decode(segments[1], &claims); err != nil {
		return nil, err
	}
	// Delete standard claims from the custom claims maps.
	for _, r := range []string{"iss", "aud", "exp", "iat", "sub", "uid"} {
		delete(claims, r)
	}
	payload.Claims = claims

	projectIDMsg := "make sure the ID token comes from the same Firebase project as the credential used to" +
		" authenticate this SDK"
	verifyTokenMsg := "see https://firebase.google.com/docs/auth/admin/verify-id-tokens for details on how to " +
		"retrieve a valid ID token"
	issuer := issuerPrefix + c.projectID

	var err error
	if header.KeyID == "" {
		if payload.Audience == firebaseAudience {
			err = fmt.Errorf("expected an ID token but got a custom token")
		} else {
			err = fmt.Errorf("ID token has no 'kid' header")
		}
	} else if header.Algorithm != "RS256" {
		err = fmt.Errorf("ID token has invalid algorithm; expected 'RS256' but got %q; %s",
			header.Algorithm, verifyTokenMsg)
	} else if payload.Audience != c.projectID {
		err = fmt.Errorf("ID token has invalid 'aud' (audience) claim; expected %q but got %q; %s; %s",
			c.projectID, payload.Audience, projectIDMsg, verifyTokenMsg)
	} else if payload.Issuer != issuer {
		err = fmt.Errorf("ID token has invalid 'iss' (issuer) claim; expected %q but got %q; %s; %s",
			issuer, payload.Issuer, projectIDMsg, verifyTokenMsg)
	} else if payload.IssuedAt > clk.Now().Unix() {
		err = fmt.Errorf("ID token issued at future timestamp: %d", payload.IssuedAt)
	} else if payload.Expires < clk.Now().Unix() {
		err = fmt.Errorf("ID token has expired at: %d", payload.Expires)
	} else if payload.Subject == "" {
		err = fmt.Errorf("ID token has empty 'sub' (subject) claim; %s", verifyTokenMsg)
	} else if len(payload.Subject) > 128 {
		err = fmt.Errorf("ID token has a 'sub' (subject) claim longer than 128 characters; %s", verifyTokenMsg)
	}

	if err != nil {
		return nil, err
	}
	payload.UID = payload.Subject
	return &payload, nil
}

// VerifyIDTokenAndCheckRevoked verifies the provided ID token and checks it has not been revoked.
//
// VerifyIDTokenAndCheckRevoked verifies the signature and payload of the provided ID token and
// checks that it wasn't revoked. Uses VerifyIDToken() internally to verify the ID token JWT.
func (c *Client) VerifyIDTokenAndCheckRevoked(ctx context.Context, idToken string) (*Token, error) {
	p, err := c.VerifyIDToken(ctx, idToken)
	if err != nil {
		return nil, err
	}

	user, err := c.GetUser(ctx, p.UID)
	if err != nil {
		return nil, err
	}

	if p.IssuedAt*1000 < user.TokensValidAfterMillis {
		return nil, internal.Error(idTokenRevoked, "ID token has been revoked")
	}
	return p, nil
}
