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
	"golang.org/x/net/context"

	"google.golang.org/appengine"
)

type aeSigner struct {
	ctx context.Context
}

func newSigner(ctx context.Context) (signer, error) {
	return aeSigner{ctx}, nil
}

func (s aeSigner) Email() (string, error) {
	return appengine.ServiceAccount(s.ctx)
}

func (s aeSigner) Sign(ss []byte) ([]byte, error) {
	_, sig, err := appengine.SignBytes(s.ctx, ss)
	return sig, err
}
