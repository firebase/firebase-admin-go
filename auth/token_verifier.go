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

package auth

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"firebase.google.com/go/internal"
	"google.golang.org/api/option"
	"google.golang.org/api/transport"
)

const (
	idTokenCertURL      = "https://www.googleapis.com/robot/v1/metadata/x509/securetoken@system.gserviceaccount.com"
	idTokenIssuerPrefix = "https://securetoken.google.com/"
	clockSkewSeconds    = 300
)

type tokenVerifier struct {
	shortName         string
	articledShortName string
	docURL            string
	projectID         string
	issuerPrefix      string
	keySource         keySource
	clock             internal.Clock
}

func newIDTokenVerifier(ctx context.Context, projectID string) (*tokenVerifier, error) {
	noAuthHTTPClient, _, err := transport.NewHTTPClient(ctx, option.WithoutAuthentication())
	if err != nil {
		return nil, err
	}
	return &tokenVerifier{
		shortName:         "ID token",
		articledShortName: "an ID token",
		docURL:            "https://firebase.google.com/docs/auth/admin/verify-id-tokens",
		projectID:         projectID,
		issuerPrefix:      idTokenIssuerPrefix,
		keySource:         newHTTPKeySource(idTokenCertURL, noAuthHTTPClient),
		clock:             internal.SystemClock,
	}, nil
}

func (tv *tokenVerifier) VerifyToken(ctx context.Context, token string) (*Token, error) {
	if tv.projectID == "" {
		return nil, errors.New("project id not available")
	}
	if token == "" {
		return nil, fmt.Errorf("%s must be a non-empty string", tv.shortName)
	}

	payload, err := tv.verifyContent(token)
	if err != nil {
		return nil, fmt.Errorf("%s; see %s for details on how to retrieve a valid %s",
			err.Error(), tv.docURL, tv.shortName)
	}

	if err := tv.verifyTimestamps(payload); err != nil {
		return nil, err
	}

	// Verifying the signature requires syncronized access to a key store and
	// potentially issues a http request. Validating the fields of the token is
	// cheaper and invalid tokens will fail faster.
	if err := tv.verifySignature(ctx, token); err != nil {
		return nil, err
	}
	return payload, nil
}

func (tv *tokenVerifier) verifyContent(token string) (*Token, error) {
	var (
		header       jwtHeader
		payload      Token
		customClaims map[string]interface{}
		err          error
	)

	segments := strings.Split(token, ".")
	if err = decode(segments[0], &header); err != nil {
		return nil, err
	}
	if err = decode(segments[1], &payload); err != nil {
		return nil, err
	}

	issuer := tv.issuerPrefix + tv.projectID
	if header.KeyID == "" {
		if payload.Audience == firebaseAudience {
			err = fmt.Errorf("expected %s but got a custom token", tv.articledShortName)
		} else {
			err = fmt.Errorf("%s has no 'kid' header", tv.shortName)
		}
	} else if header.Algorithm != "RS256" {
		err = fmt.Errorf("%s has invalid algorithm; expected 'RS256' but got %q",
			tv.shortName, header.Algorithm)
	} else if payload.Audience != tv.projectID {
		err = fmt.Errorf("%s has invalid 'aud' (audience) claim; expected %q but got %q; %s",
			tv.shortName, tv.projectID, payload.Audience, tv.getProjectIDMatchMessage())
	} else if payload.Issuer != issuer {
		err = fmt.Errorf("%s has invalid 'iss' (issuer) claim; expected %q but got %q; %s",
			tv.shortName, issuer, payload.Issuer, tv.getProjectIDMatchMessage())
	} else if payload.Subject == "" {
		err = fmt.Errorf("%s has empty 'sub' (subject) claim", tv.shortName)
	} else if len(payload.Subject) > 128 {
		err = fmt.Errorf("%s has a 'sub' (subject) claim longer than 128 characters", tv.shortName)
	}

	if err != nil {
		return nil, err
	}

	payload.UID = payload.Subject
	if err = decode(segments[1], &customClaims); err != nil {
		return nil, err
	}
	for _, standardClaim := range []string{"iss", "aud", "exp", "iat", "sub", "uid"} {
		delete(customClaims, standardClaim)
	}
	payload.Claims = customClaims
	return &payload, nil
}

func (tv *tokenVerifier) verifyTimestamps(payload *Token) error {
	if (payload.IssuedAt - clockSkewSeconds) > tv.clock.Now().Unix() {
		return fmt.Errorf("%s issued at future timestamp: %d", tv.shortName, payload.IssuedAt)
	} else if (payload.Expires + clockSkewSeconds) < tv.clock.Now().Unix() {
		return fmt.Errorf("%s has expired at: %d", tv.shortName, payload.Expires)
	}
	return nil
}

func (tv *tokenVerifier) verifySignature(ctx context.Context, token string) error {
	segments := strings.Split(token, ".")

	var h jwtHeader
	if err := decode(segments[0], &h); err != nil {
		return err
	}

	keys, err := tv.keySource.Keys(ctx)
	if err != nil {
		return err
	}

	verified := false
	for _, k := range keys {
		if h.KeyID == "" || h.KeyID == k.Kid {
			if verifySignature(segments, k) == nil {
				verified = true
				break
			}
		}
	}

	if !verified {
		return errors.New("failed to verify token signature")
	}
	return nil
}

func (tv *tokenVerifier) getProjectIDMatchMessage() string {
	return fmt.Sprintf(
		"make sure the %s comes from the same Firebase project as the credential used to"+
			" authenticate this SDK", tv.shortName)
}

// keySource is used to obtain a set of public keys, which can be used to verify cryptographic
// signatures.
type keySource interface {
	Keys(context.Context) ([]*publicKey, error)
}

// httpKeySource fetches RSA public keys from a remote HTTP server, and caches them in
// memory. It also handles cache! invalidation and refresh based on the standard HTTP
// cache-control headers.
type httpKeySource struct {
	KeyURI     string
	HTTPClient *http.Client
	CachedKeys []*publicKey
	ExpiryTime time.Time
	Clock      internal.Clock
	Mutex      *sync.Mutex
}

func newHTTPKeySource(uri string, hc *http.Client) *httpKeySource {
	return &httpKeySource{
		KeyURI:     uri,
		HTTPClient: hc,
		Clock:      internal.SystemClock,
		Mutex:      &sync.Mutex{},
	}
}

// Keys returns the RSA Public Keys hosted at this key source's URI. Refreshes the data if
// the cache is stale.
func (k *httpKeySource) Keys(ctx context.Context) ([]*publicKey, error) {
	k.Mutex.Lock()
	defer k.Mutex.Unlock()
	if len(k.CachedKeys) == 0 || k.hasExpired() {
		err := k.refreshKeys(ctx)
		if err != nil && len(k.CachedKeys) == 0 {
			return nil, err
		}
	}
	return k.CachedKeys, nil
}

// hasExpired indicates whether the cache has expired.
func (k *httpKeySource) hasExpired() bool {
	return k.Clock.Now().After(k.ExpiryTime)
}

func (k *httpKeySource) refreshKeys(ctx context.Context) error {
	k.CachedKeys = nil
	req, err := http.NewRequest("GET", k.KeyURI, nil)
	if err != nil {
		return err
	}

	resp, err := k.HTTPClient.Do(req.WithContext(ctx))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	contents, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("invalid response (%d) while retrieving public keys: %s", resp.StatusCode, string(contents))
	}
	newKeys, err := parsePublicKeys(contents)
	if err != nil {
		return err
	}
	maxAge, err := findMaxAge(resp)
	if err != nil {
		return err
	}
	k.CachedKeys = append([]*publicKey(nil), newKeys...)
	k.ExpiryTime = k.Clock.Now().Add(*maxAge)
	return nil
}

// decode accepts a JWT segment, and decodes it into the given interface.
func decode(segment string, i interface{}) error {
	decoded, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil {
		return err
	}
	return json.NewDecoder(bytes.NewBuffer(decoded)).Decode(i)
}

func verifySignature(parts []string, k *publicKey) error {
	content := parts[0] + "." + parts[1]
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return err
	}

	h := sha256.New()
	h.Write([]byte(content))
	return rsa.VerifyPKCS1v15(k.Key, crypto.SHA256, h.Sum(nil), []byte(signature))
}

// publicKey represents a parsed RSA public key along with its unique key ID.
type publicKey struct {
	Kid string
	Key *rsa.PublicKey
}

func findMaxAge(resp *http.Response) (*time.Duration, error) {
	cc := resp.Header.Get("cache-control")
	for _, value := range strings.Split(cc, ",") {
		value = strings.TrimSpace(value)
		if strings.HasPrefix(value, "max-age=") {
			sep := strings.Index(value, "=")
			seconds, err := strconv.ParseInt(value[sep+1:], 10, 64)
			if err != nil {
				return nil, err
			}
			duration := time.Duration(seconds) * time.Second
			return &duration, nil
		}
	}
	return nil, errors.New("Could not find expiry time from HTTP headers")
}

func parsePublicKeys(keys []byte) ([]*publicKey, error) {
	m := make(map[string]string)
	err := json.Unmarshal(keys, &m)
	if err != nil {
		return nil, err
	}

	var result []*publicKey
	for kid, key := range m {
		pubKey, err := parsePublicKey(kid, []byte(key))
		if err != nil {
			return nil, err
		}
		result = append(result, pubKey)
	}
	return result, nil
}

func parsePublicKey(kid string, key []byte) (*publicKey, error) {
	block, _ := pem.Decode(key)
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	pk, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("Certificate is not a RSA key")
	}
	return &publicKey{kid, pk}, nil
}
