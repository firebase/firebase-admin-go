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
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
)

type serviceAcctSigner struct {
	email string
	pk    *rsa.PrivateKey
}

func (s serviceAcctSigner) Email() (string, error) {
	if s.email == "" {
		return "", errors.New("service account email not available")
	}
	return s.email, nil
}

func (s serviceAcctSigner) Sign(ss []byte) ([]byte, error) {
	if s.pk == nil {
		return nil, errors.New("private key not available")
	}
	hash := sha256.New()
	hash.Write([]byte(ss))
	return rsa.SignPKCS1v15(rand.Reader, s.pk, crypto.SHA256, hash.Sum(nil))
}

func parseKey(key string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(key))
	if block == nil {
		return nil, fmt.Errorf("no private key data found in: %v", key)
	}
	k := block.Bytes
	parsedKey, err := x509.ParsePKCS8PrivateKey(k)
	if err != nil {
		parsedKey, err = x509.ParsePKCS1PrivateKey(k)
		if err != nil {
			return nil, fmt.Errorf("private key should be a PEM or plain PKSC1 or PKCS8; parse error: %v", err)
		}
	}
	parsed, ok := parsedKey.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not an RSA key")
	}
	return parsed, nil
}
