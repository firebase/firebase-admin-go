//go:build appengine
// +build appengine

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

	// "firebase.google.com/go/v4/internal" // No longer needed directly by newCryptoSigner
	"google.golang.org/api/option" // To match signature of non-AppEngine version
	"google.golang.org/appengine/v2"
)

type aeSigner struct{}

// conf parameter (serviceAccountID, sdkVersion, opts) is unused in App Engine implementation but included for signature consistency.
func newCryptoSigner(ctx context.Context, serviceAccountID string, sdkVersion string, opts ...option.ClientOption) (cryptoSigner, error) {
	return aeSigner{}, nil
}

func (s aeSigner) Email(ctx context.Context) (string, error) {
	return appengine.ServiceAccount(ctx)
}

func (s aeSigner) Sign(ctx context.Context, b []byte) ([]byte, error) {
	_, sig, err := appengine.SignBytes(ctx, b)
	return sig, err
}
