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
	"reflect"
	"testing"

	"firebase.google.com/go/auth"
	"firebase.google.com/go/internal"
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
		alg:  Bcrypt{},
		want: internal.HashConfig{HashAlgorithm: "BCRYPT"},
	},
	{
		alg: StandardScrypt{
			BlockSize:        1,
			DerivedKeyLength: 2,
			Parallelization:  3,
			MemoryCost:       4,
		},
		want: internal.HashConfig{
			HashAlgorithm:    "STANDARD_SCRYPT",
			BlockSize:        1,
			DerivedKeyLength: 2,
			Parallelization:  3,
			MemoryCost:       4,
			ForceSendFields:  []string{"BlockSize", "Parallelization", "MemoryCost", "DkLen"},
		},
	},
	{
		alg: Scrypt{
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
	{
		alg: HMACMD5{signerKey},
		want: internal.HashConfig{
			HashAlgorithm: "HMAC_MD5",
			SignerKey:     base64.RawURLEncoding.EncodeToString(signerKey),
		},
	},
	{
		alg: HMACSHA1{signerKey},
		want: internal.HashConfig{
			HashAlgorithm: "HMAC_SHA1",
			SignerKey:     base64.RawURLEncoding.EncodeToString(signerKey),
		},
	},
	{
		alg: HMACSHA256{signerKey},
		want: internal.HashConfig{
			HashAlgorithm: "HMAC_SHA256",
			SignerKey:     base64.RawURLEncoding.EncodeToString(signerKey),
		},
	},
	{
		alg: HMACSHA512{signerKey},
		want: internal.HashConfig{
			HashAlgorithm: "HMAC_SHA512",
			SignerKey:     base64.RawURLEncoding.EncodeToString(signerKey),
		},
	},
	{
		alg: MD5{42},
		want: internal.HashConfig{
			HashAlgorithm:   "MD5",
			Rounds:          42,
			ForceSendFields: []string{"Rounds"},
		},
	},
	{
		alg: SHA1{42},
		want: internal.HashConfig{
			HashAlgorithm:   "SHA1",
			Rounds:          42,
			ForceSendFields: []string{"Rounds"},
		},
	},
	{
		alg: SHA256{42},
		want: internal.HashConfig{
			HashAlgorithm:   "SHA256",
			Rounds:          42,
			ForceSendFields: []string{"Rounds"},
		},
	},
	{
		alg: SHA512{42},
		want: internal.HashConfig{
			HashAlgorithm:   "SHA512",
			Rounds:          42,
			ForceSendFields: []string{"Rounds"},
		},
	},
	{
		alg: PBKDFSHA1{42},
		want: internal.HashConfig{
			HashAlgorithm:   "PBKDF_SHA1",
			Rounds:          42,
			ForceSendFields: []string{"Rounds"},
		},
	},
	{
		alg: PBKDF2SHA256{42},
		want: internal.HashConfig{
			HashAlgorithm:   "PBKDF2_SHA256",
			Rounds:          42,
			ForceSendFields: []string{"Rounds"},
		},
	},
}

var invalidHashes = []struct {
	name string
	alg  auth.UserImportHash
}{
	{
		name: "SCRYPT: no signer key",
		alg: Scrypt{
			SaltSeparator: saltSeparator,
			Rounds:        8,
			MemoryCost:    14,
		},
	},
	{
		name: "SCRYPT: low rounds",
		alg: Scrypt{
			Key:           signerKey,
			SaltSeparator: saltSeparator,
			MemoryCost:    14,
		},
	},
	{
		name: "SCRYPT: high rounds",
		alg: Scrypt{
			Key:           signerKey,
			SaltSeparator: saltSeparator,
			Rounds:        9,
			MemoryCost:    14,
		},
	},
	{
		name: "SCRYPT: low memory cost",
		alg: Scrypt{
			Key:           signerKey,
			SaltSeparator: saltSeparator,
			Rounds:        8,
		},
	},
	{
		name: "SCRYPT: high memory cost",
		alg: Scrypt{
			Key:           signerKey,
			SaltSeparator: saltSeparator,
			Rounds:        8,
			MemoryCost:    15,
		},
	},
	{
		name: "HMAC_MD5: no signer key",
		alg:  HMACMD5{},
	},
	{
		name: "HMAC_SHA1: no signer key",
		alg:  HMACSHA1{},
	},
	{
		name: "HMAC_SHA256: no signer key",
		alg:  HMACSHA256{},
	},
	{
		name: "HMAC_SHA512: no signer key",
		alg:  HMACSHA512{},
	},
	{
		name: "MD5: rounds too low",
		alg:  MD5{-1},
	},
	{
		name: "SHA1: rounds too low",
		alg:  SHA1{-1},
	},
	{
		name: "SHA256: rounds too low",
		alg:  SHA256{-1},
	},
	{
		name: "SHA512: rounds too low",
		alg:  SHA512{-1},
	},
	{
		name: "PBKDFSHA1: rounds too low",
		alg:  PBKDFSHA1{-1},
	},
	{
		name: "PBKDF2SHA256: rounds too low",
		alg:  PBKDF2SHA256{-1},
	},
	{
		name: "MD5: rounds too high",
		alg:  MD5{120001},
	},
	{
		name: "SHA1: rounds too high",
		alg:  SHA1{120001},
	},
	{
		name: "SHA256: rounds too high",
		alg:  SHA256{120001},
	},
	{
		name: "SHA512: rounds too high",
		alg:  SHA512{120001},
	},
	{
		name: "PBKDFSHA1: rounds too high",
		alg:  PBKDFSHA1{120001},
	},
	{
		name: "PBKDF2SHA256: rounds too high",
		alg:  PBKDF2SHA256{120001},
	},
}

func TestValidHash(t *testing.T) {
	for idx, tc := range validHashes {
		got, err := tc.alg.Config()
		if err != nil {
			t.Errorf("[%d] Config() = %v", idx, err)
		} else if !reflect.DeepEqual(*got, tc.want) {
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
