// +build !appengine

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
	"testing"

	"firebase.google.com/go/internal"
)

func TestCustomTokenInvalidCredential(t *testing.T) {
	s, err := NewClient(&internal.AuthConfig{Ctx: context.Background()})
	if err != nil {
		t.Fatal(err)
	}

	token, err := s.CustomToken("user1")
	if token != "" || err == nil {
		t.Errorf("CustomTokenWithClaims() = (%q, %v); want: (\"\", error)", token, err)
	}

	token, err = s.CustomTokenWithClaims("user1", map[string]interface{}{"foo": "bar"})
	if token != "" || err == nil {
		t.Errorf("CustomTokenWithClaims() = (%q, %v); want: (\"\", error)", token, err)
	}
}

func TestNoProjectID(t *testing.T) {
	c, err := NewClient(&internal.AuthConfig{Ctx: context.Background(), Creds: creds})
	if err != nil {
		t.Fatal(err)
	}
	c.ks = client.ks
	if _, err := c.VerifyIDToken(testIDToken); err == nil {
		t.Error("VeridyIDToken() = nil; want error")
	}
}

func TestVerifyIDToken(t *testing.T) {
	ft, err := client.VerifyIDToken(testIDToken)
	if err != nil {
		t.Fatal(err)
	}
	if ft.Claims["admin"] != true {
		t.Errorf("Claims['admin'] = %v; want: true", ft.Claims["admin"])
	}
	if ft.UID != ft.Subject {
		t.Errorf("UID = %q; Sub = %q; want UID = Sub", ft.UID, ft.Subject)
	}
}
