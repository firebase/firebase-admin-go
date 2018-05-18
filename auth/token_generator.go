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
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"

	"golang.org/x/net/context"
)

type jwtHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
	KeyID     string `json:"kid,omitempty"`
}

type customToken struct {
	Iss    string                 `json:"iss"`
	Aud    string                 `json:"aud"`
	Exp    int64                  `json:"exp"`
	Iat    int64                  `json:"iat"`
	Sub    string                 `json:"sub,omitempty"`
	UID    string                 `json:"uid,omitempty"`
	Claims map[string]interface{} `json:"claims,omitempty"`
}

type jwtInfo struct {
	header  jwtHeader
	payload interface{}
}

func (info *jwtInfo) Token(ctx context.Context, signer cryptoSigner) (string, error) {
	encode := func(i interface{}) (string, error) {
		b, err := json.Marshal(i)
		if err != nil {
			return "", err
		}
		return base64.RawURLEncoding.EncodeToString(b), nil
	}
	header, err := encode(info.header)
	if err != nil {
		return "", err
	}
	payload, err := encode(info.payload)
	if err != nil {
		return "", err
	}

	tokenData := fmt.Sprintf("%s.%s", header, payload)
	sig, err := signer.Sign(ctx, []byte(tokenData))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.%s", tokenData, base64.RawURLEncoding.EncodeToString(sig)), nil
}

type serviceAccount struct {
	PrivateKey  string `json:"private_key"`
	ClientEmail string `json:"client_email"`
}

// cryptoSigner is used to cryptographically sign data, and query the identity of the signer.
type cryptoSigner interface {
	Sign(context.Context, []byte) ([]byte, error)
	Email(context.Context) (string, error)
}

type serviceAccountSigner struct {
	privateKey  *rsa.PrivateKey
	clientEmail string
}

func newServiceAccountSigner(sa serviceAccount) (*serviceAccountSigner, error) {
	block, _ := pem.Decode([]byte(sa.PrivateKey))
	if block == nil {
		return nil, fmt.Errorf("no private key data found in: %q", sa.PrivateKey)
	}
	parsedKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		parsedKey, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("private key should be a PEM or plain PKSC1 or PKCS8; parse error: %v", err)
		}
	}
	rsaKey, ok := parsedKey.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not an RSA key")
	}
	return &serviceAccountSigner{
		privateKey:  rsaKey,
		clientEmail: sa.ClientEmail,
	}, nil
}

func (s serviceAccountSigner) Sign(ctx context.Context, b []byte) ([]byte, error) {
	hash := sha256.New()
	hash.Write(b)
	return rsa.SignPKCS1v15(rand.Reader, s.privateKey, crypto.SHA256, hash.Sum(nil))
}

func (s serviceAccountSigner) Email(ctx context.Context) (string, error) {
	return s.clientEmail, nil
}

type iamSigner struct{}

func (s iamSigner) Sign(ctx context.Context, b []byte) ([]byte, error) {
	return nil, errors.New("not implemented")
}

func (s iamSigner) Email(ctx context.Context) (string, error) {
	return "", errors.New("not implemented")
}
