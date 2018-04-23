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

// Package hash contains a collection of password hash algorithms that can be used with the
// auth.ImportUsers() API.
package hash // import "firebase.google.com/go/auth/hash"

import (
	"encoding/base64"
	"errors"

	"firebase.google.com/go/internal"
)

// Scrypt represents the SCRYPT hash algorithm.
type Scrypt struct {
	Key           []byte
	SaltSeparator []byte
	Rounds        int
	MemoryCost    int
}

// Config returns the validated hash configuration.
func (s *Scrypt) Config() (*internal.HashConfig, error) {
	if len(s.Key) == 0 {
		return nil, errors.New("signer key not specified")
	}
	if s.Rounds < 1 || s.Rounds > 8 {
		return nil, errors.New("rounds must be between 1 and 8")
	}
	if s.MemoryCost < 1 || s.MemoryCost > 14 {
		return nil, errors.New("memory cost must be between 1 and 14")
	}
	return &internal.HashConfig{
		HashAlgorithm: "SCRYPT",
		SignerKey:     base64.RawURLEncoding.EncodeToString(s.Key),
		SaltSeparator: base64.RawURLEncoding.EncodeToString(s.SaltSeparator),
		Rounds:        int64(s.Rounds),
		MemoryCost:    int64(s.MemoryCost),
	}, nil
}

// HMACSHA512 represents the HMAC SHA512 hash algorithm.
type HMACSHA512 struct {
	Key []byte
}

// Config returns the validated hash configuration.
func (h *HMACSHA512) Config() (*internal.HashConfig, error) {
	return hmacConfig("HMAC_SHA512", h.Key)
}

func hmacConfig(name string, key []byte) (*internal.HashConfig, error) {
	if len(key) == 0 {
		return nil, errors.New("signer key not specified")
	}
	return &internal.HashConfig{
		HashAlgorithm: name,
		SignerKey:     base64.RawURLEncoding.EncodeToString(key),
	}, nil
}
