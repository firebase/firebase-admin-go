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

	"google.golang.org/api/option"
	"google.golang.org/appengine"
)

type aeKeySource struct {
	keys []*publicKey
}

func newKeySource(ctx context.Context, uri string, opts ...option.ClientOption) (keySource, error) {
	certs, err := appengine.PublicCertificates(ctx)
	if err != nil {
		return nil, err
	}
	keys := make([]*publicKey, len(certs))
	for i, cert := range certs {
		pk, err := parsePublicKey(cert.KeyName, cert.Data)
		if err != nil {
			return nil, err
		}
		keys[i] = pk
	}
	return aeKeySource{keys}, nil

}

// Keys returns the RSA Public Keys managed by App Engine.
func (k aeKeySource) Keys() ([]*publicKey, error) {
	return k.keys, nil
}
