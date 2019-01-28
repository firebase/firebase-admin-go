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

package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"firebase.google.com/go/internal"
)

func TestEncodeToken(t *testing.T) {
	info := &jwtInfo{
		header:  jwtHeader{Algorithm: "RS256", Type: "JWT"},
		payload: mockIDTokenPayload{"key": "value"},
	}
	s, err := info.Token(context.Background(), &mockSigner{})
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		t.Errorf("encodeToken() = %d; want: %d", len(parts), 3)
	}

	var header jwtHeader
	if err := decode(parts[0], &header); err != nil {
		t.Fatal(err)
	} else if info.header != header {
		t.Errorf("decode(header) = %v; want = %v", header, info.header)
	}

	payload := make(mockIDTokenPayload)
	if err := decode(parts[1], &payload); err != nil {
		t.Fatal(err)
	} else if len(payload) != 1 || payload["key"] != "value" {
		t.Errorf("decode(payload) = %v; want = %v", payload, info.payload)
	}

	if sig, err := base64.RawURLEncoding.DecodeString(parts[2]); err != nil {
		t.Fatal(err)
	} else if string(sig) != "signature" {
		t.Errorf("decode(signature) = %q; want = %q", string(sig), "signature")
	}
}

func TestEncodeSignError(t *testing.T) {
	signer := &mockSigner{
		err: errors.New("sign error"),
	}
	info := &jwtInfo{
		header:  jwtHeader{Algorithm: "RS256", Type: "JWT"},
		payload: mockIDTokenPayload{"key": "value"},
	}
	if s, err := info.Token(context.Background(), signer); s != "" || err != signer.err {
		t.Errorf("encodeToken() = (%v, %v); want = ('', %v)", s, err, signer.err)
	}
}

func TestEncodeInvalidPayload(t *testing.T) {
	info := &jwtInfo{
		header:  jwtHeader{Algorithm: "RS256", Type: "JWT"},
		payload: mockIDTokenPayload{"key": func() {}},
	}
	s, err := info.Token(context.Background(), &mockSigner{})
	if s != "" || err == nil {
		t.Errorf("encodeToken() = (%v, %v); want = ('', error)", s, err)
	}
}

func TestServiceAccountSigner(t *testing.T) {
	b, err := ioutil.ReadFile("../testdata/service_account.json")
	if err != nil {
		t.Fatal(err)
	}

	var sa serviceAccount
	if err := json.Unmarshal(b, &sa); err != nil {
		t.Fatal(err)
	}
	signer, err := newServiceAccountSigner(sa)
	if err != nil {
		t.Fatal(err)
	}
	email, err := signer.Email(context.Background())
	if email != sa.ClientEmail || err != nil {
		t.Errorf("Email() = (%q, %v); want = (%q, nil)", email, err, sa.ClientEmail)
	}
	sign, err := signer.Sign(context.Background(), []byte("test"))
	if sign == nil || err != nil {
		t.Errorf("Sign() = (%v, %v); want = (bytes, nil)", email, err)
	}
}

func TestIAMSigner(t *testing.T) {
	ctx := context.Background()
	conf := &internal.AuthConfig{
		Opts:             optsWithTokenSource,
		ServiceAccountID: "test-service-account",
	}
	signer, err := newIAMSigner(ctx, conf)
	if err != nil {
		t.Fatal(err)
	}
	email, err := signer.Email(ctx)
	if email != conf.ServiceAccountID || err != nil {
		t.Errorf("Email() = (%q, %v); want = (%q, nil)", email, err, conf.ServiceAccountID)
	}

	wantSignature := "test-signature"
	server := iamServer(t, email, wantSignature)
	defer server.Close()
	signer.iamHost = server.URL

	signature, err := signer.Sign(ctx, []byte("input"))
	if err != nil {
		t.Fatal(err)
	}
	if string(signature) != wantSignature {
		t.Errorf("Sign() = %q; want = %q", string(signature), wantSignature)
	}
}

func TestIAMSignerHTTPError(t *testing.T) {
	conf := &internal.AuthConfig{
		Opts:             optsWithTokenSource,
		ServiceAccountID: "test-service-account",
	}
	signer, err := newIAMSigner(context.Background(), conf)
	if err != nil {
		t.Fatal(err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		w.WriteHeader(http.StatusForbidden)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"error": {"status": "PERMISSION_DENIED", "message": "test reason"}}`))
	})
	server := httptest.NewServer(handler)
	defer server.Close()
	signer.iamHost = server.URL

	want := "http error status: 403; reason: test reason"
	_, err = signer.Sign(context.Background(), []byte("input"))
	if err == nil || !IsInsufficientPermission(err) || err.Error() != want {
		t.Errorf("Sign() = %v; want = %q", err, want)
	}
}

func TestIAMSignerUnknownHTTPError(t *testing.T) {
	conf := &internal.AuthConfig{
		Opts:             optsWithTokenSource,
		ServiceAccountID: "test-service-account",
	}
	signer, err := newIAMSigner(context.Background(), conf)
	if err != nil {
		t.Fatal(err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		w.WriteHeader(http.StatusForbidden)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`not json`))
	})
	server := httptest.NewServer(handler)
	defer server.Close()
	signer.iamHost = server.URL

	want := "http error status: 403; reason: client encountered an unknown error; response: not json"
	_, err = signer.Sign(context.Background(), []byte("input"))
	if err == nil || !IsUnknown(err) || err.Error() != want {
		t.Errorf("Sign() = %v; want = %q", err, want)
	}
}

func TestIAMSignerWithMetadataService(t *testing.T) {
	ctx := context.Background()
	conf := &internal.AuthConfig{
		Opts: optsWithTokenSource,
	}

	signer, err := newIAMSigner(ctx, conf)
	if err != nil {
		t.Fatal(err)
	}

	// start mock metadata service and test Email()
	serviceAcct := "discovered-service-account"
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		flavor := r.Header.Get("Metadata-Flavor")
		if flavor != "Google" {
			t.Errorf("Header(Metadata-Flavor) = %q; want = %q", flavor, "Google")
		}
		w.Header().Set("Content-Type", "application/text")
		w.Write([]byte(serviceAcct))
	})
	metadata := httptest.NewServer(handler)
	defer metadata.Close()
	signer.metadataHost = metadata.URL
	email, err := signer.Email(ctx)
	if email != serviceAcct || err != nil {
		t.Errorf("Email() = (%q, %v); want = (%q, nil)", email, err, serviceAcct)
	}

	// start mock IAM service and test Sign()
	wantSignature := "test-signature"
	server := iamServer(t, email, wantSignature)
	defer server.Close()
	signer.iamHost = server.URL

	signature, err := signer.Sign(ctx, []byte("input"))
	if err != nil {
		t.Fatal(err)
	}
	if string(signature) != wantSignature {
		t.Errorf("Sign() = %q; want = %q", string(signature), wantSignature)
	}
}

func TestIAMSignerNoMetadataService(t *testing.T) {
	ctx := context.Background()
	conf := &internal.AuthConfig{
		Opts: optsWithTokenSource,
	}

	signer, err := newIAMSigner(ctx, conf)
	if err != nil {
		t.Fatal(err)
	}

	if _, err = signer.Email(ctx); err == nil {
		t.Errorf("Email() = nil; want = error")
	}
	if _, err = signer.Sign(ctx, []byte("input")); err == nil {
		t.Errorf("Sign() = nil; want = error")
	}
}

type mockSigner struct {
	err error
}

func (s *mockSigner) Email(ctx context.Context) (string, error) {
	return "", nil
}

func (s *mockSigner) Sign(ctx context.Context, b []byte) ([]byte, error) {
	if s.err != nil {
		return nil, s.err
	}
	return []byte("signature"), nil
}

func iamServer(t *testing.T, serviceAcct, signature string) *httptest.Server {
	resp := map[string]interface{}{
		"signature": base64.StdEncoding.EncodeToString([]byte(signature)),
	}
	wantPath := fmt.Sprintf("/v1/projects/-/serviceAccounts/%s:signBlob", serviceAcct)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		reqBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]interface{}
		if err := json.Unmarshal(reqBody, &m); err != nil {
			t.Fatal(err)
		}
		if m["bytesToSign"] == "" {
			t.Fatal("BytesToSign = empty; want = non-empty")
		}
		if r.URL.Path != wantPath {
			t.Errorf("Path = %q; want = %q", r.URL.Path, wantPath)
		}

		w.Header().Set("Content-Type", "application/json")
		b, err := json.Marshal(resp)
		if err != nil {
			t.Fatal(err)
		}
		w.Write(b)
	})
	return httptest.NewServer(handler)
}
