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
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"golang.org/x/net/context"
)

func TestEncodeToken(t *testing.T) {
	h := jwtHeader{Algorithm: "RS256", Type: "JWT"}
	p := mockIDTokenPayload{"key": "value"}
	s, err := encodeToken(ctx, &mockSigner{}, h, p)
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
	} else if h != header {
		t.Errorf("decode(header) = %v; want = %v", header, h)
	}

	payload := make(mockIDTokenPayload)
	if err := decode(parts[1], &payload); err != nil {
		t.Fatal(err)
	} else if len(payload) != 1 || payload["key"] != "value" {
		t.Errorf("decode(payload) = %v; want = %v", payload, p)
	}

	if sig, err := base64.RawURLEncoding.DecodeString(parts[2]); err != nil {
		t.Fatal(err)
	} else if string(sig) != "signature" {
		t.Errorf("decode(signature) = %q; want = %q", string(sig), "signature")
	}
}

func TestEncodeSignError(t *testing.T) {
	h := jwtHeader{Algorithm: "RS256", Type: "JWT"}
	p := mockIDTokenPayload{"key": "value"}
	signer := &mockSigner{
		err: errors.New("sign error"),
	}
	if s, err := encodeToken(ctx, signer, h, p); s != "" || err == nil {
		t.Errorf("encodeToken() = (%v, %v); want = ('', error)", s, err)
	}
}

func TestEncodeInvalidPayload(t *testing.T) {
	h := jwtHeader{Algorithm: "RS256", Type: "JWT"}
	p := mockIDTokenPayload{"key": func() {}}
	if s, err := encodeToken(ctx, &mockSigner{}, h, p); s != "" || err == nil {
		t.Errorf("encodeToken() = (%v, %v); want = ('', error)", s, err)
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
