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

package auth // import "firebase.google.com/go/auth"

import (
	"firebase.google.com/go/internal"
	"golang.org/x/net/context"
)

func newCryptoSigner(ctx context.Context, conf *internal.AuthConfig) (cryptoSigner, error) {
	return newIAMSigner(ctx, conf)
}
