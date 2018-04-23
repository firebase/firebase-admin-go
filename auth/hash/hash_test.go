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

package hash

import (
	"encoding/base64"
	"testing"

	"firebase.google.com/go/internal"

	"firebase.google.com/go/auth"
)

var (
	signerKey     = []byte("key")
	saltSeparator = []byte("sep")
)

var validHashes = []struct {
	alg  auth.UserImportHash
	want internal.HashConfig
}{
	{
		alg: &Scrypt{
			Key:           signerKey,
			SaltSeparator: saltSeparator,
			Rounds:        8,
			MemoryCost:    14,
		},
		want: internal.HashConfig{
			HashAlgorithm: "SCRYPT",
			SignerKey:     base64.RawURLEncoding.EncodeToString(signerKey),
			SaltSeparator: base64.RawURLEncoding.EncodeToString(saltSeparator),
			Rounds:        8,
			MemoryCost:    14,
		},
	},
}

var invalidHashes = []struct {
	name string
	alg  auth.UserImportHash
}{
	{
		name: "SCRYPT: no signer key",
		alg: &Scrypt{
			SaltSeparator: saltSeparator,
			Rounds:        8,
			MemoryCost:    14,
		},
	},
	{
		name: "SCRYPT: low rounds",
		alg: &Scrypt{
			Key:           signerKey,
			SaltSeparator: saltSeparator,
			MemoryCost:    14,
		},
	},
	{
		name: "SCRYPT: high rounds",
		alg: &Scrypt{
			Key:           signerKey,
			SaltSeparator: saltSeparator,
			Rounds:        9,
			MemoryCost:    14,
		},
	},
	{
		name: "SCRYPT: low memory cost",
		alg: &Scrypt{
			Key:           signerKey,
			SaltSeparator: saltSeparator,
			Rounds:        8,
		},
	},
	{
		name: "SCRYPT: high memory cost",
		alg: &Scrypt{
			Key:           signerKey,
			SaltSeparator: saltSeparator,
			Rounds:        8,
			MemoryCost:    15,
		},
	},
}

func TestValidHash(t *testing.T) {
	for idx, tc := range validHashes {
		got, err := tc.alg.Config()
		if err != nil {
			t.Errorf("[%d] Config() = %v", idx, err)
		} else if *got != tc.want {
			t.Errorf("[%d] Config() = %#v; want = %#v", idx, got, tc.want)
		}
	}
}

func TestInvalidHash(t *testing.T) {
	for _, tc := range invalidHashes {
		got, err := tc.alg.Config()
		if got != nil || err == nil {
			t.Errorf("%s; Config() = (%v, %v); want = (nil, error)", tc.name, got, err)
		}
	}
}
