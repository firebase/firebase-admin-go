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
// auth.ImportUsers() API. Refer to https://firebase.google.com/docs/auth/admin/import-users for
// more details about supported hash algorithms.
package hash

import (
	"encoding/base64"
	"errors"
	"fmt"

	"firebase.google.com/go/v4/internal"
)

// InputOrderType specifies the order in which users' passwords/salts are hashed
type InputOrderType int

// Available InputOrderType values
const (
	InputOrderUnspecified InputOrderType = iota
	InputOrderSaltFirst
	InputOrderPasswordFirst
)

// Bcrypt represents the BCRYPT hash algorithm.
//
// Refer to https://firebase.google.com/docs/auth/admin/import-users#import_users_with_bcrypt_hashed_passwords
// for more details.
type Bcrypt struct{}

// Config returns the validated hash configuration.
func (b Bcrypt) Config() (internal.HashConfig, error) {
	return internal.HashConfig{"hashAlgorithm": "BCRYPT"}, nil
}

// StandardScrypt represents the standard scrypt hash algorithm.
//
// Refer to https://firebase.google.com/docs/auth/admin/import-users#import_users_with_standard_scrypt_hashed_passwords
// for more details.
type StandardScrypt struct {
	BlockSize        int
	DerivedKeyLength int
	MemoryCost       int
	Parallelization  int
}

// Config returns the validated hash configuration.
func (s StandardScrypt) Config() (internal.HashConfig, error) {
	return internal.HashConfig{
		"hashAlgorithm":   "STANDARD_SCRYPT",
		"dkLen":           s.DerivedKeyLength,
		"blockSize":       s.BlockSize,
		"parallelization": s.Parallelization,
		"memoryCost":      s.MemoryCost,
	}, nil
}

// Scrypt represents the scrypt hash algorithm.
//
// This is the modified scrypt used by Firebase Auth (https://github.com/firebase/scrypt).
// Rounds must be between 1 and 8, and the MemoryCost must be between 1 and 14. Key is required.
// Refer to https://firebase.google.com/docs/auth/admin/import-users#import_users_with_firebase_scrypt_hashed_passwords
// for more details.
type Scrypt struct {
	Key           []byte
	SaltSeparator []byte
	Rounds        int
	MemoryCost    int
}

// Config returns the validated hash configuration.
func (s Scrypt) Config() (internal.HashConfig, error) {
	if len(s.Key) == 0 {
		return nil, errors.New("signer key not specified")
	}
	if s.Rounds < 1 || s.Rounds > 8 {
		return nil, errors.New("rounds must be between 1 and 8")
	}
	if s.MemoryCost < 1 || s.MemoryCost > 14 {
		return nil, errors.New("memory cost must be between 1 and 14")
	}
	return internal.HashConfig{
		"hashAlgorithm": "SCRYPT",
		"signerKey":     base64.RawURLEncoding.EncodeToString(s.Key),
		"saltSeparator": base64.RawURLEncoding.EncodeToString(s.SaltSeparator),
		"rounds":        s.Rounds,
		"memoryCost":    s.MemoryCost,
	}, nil
}

// HMACMD5 represents the HMAC SHA512 hash algorithm.
//
// Refer to https://firebase.google.com/docs/auth/admin/import-users#import_users_with_hmac_hashed_passwords
// for more details. Key is required.
type HMACMD5 struct {
	Key        []byte
	InputOrder InputOrderType
}

// Config returns the validated hash configuration.
func (h HMACMD5) Config() (internal.HashConfig, error) {
	return hmacConfig("HMAC_MD5", h.Key, h.InputOrder)
}

// HMACSHA1 represents the HMAC SHA512 hash algorithm.
//
// Key is required.
// Refer to https://firebase.google.com/docs/auth/admin/import-users#import_users_with_hmac_hashed_passwords
// for more details.
type HMACSHA1 struct {
	Key        []byte
	InputOrder InputOrderType
}

// Config returns the validated hash configuration.
func (h HMACSHA1) Config() (internal.HashConfig, error) {
	return hmacConfig("HMAC_SHA1", h.Key, h.InputOrder)
}

// HMACSHA256 represents the HMAC SHA512 hash algorithm.
//
// Key is required.
// Refer to https://firebase.google.com/docs/auth/admin/import-users#import_users_with_hmac_hashed_passwords
// for more details.
type HMACSHA256 struct {
	Key        []byte
	InputOrder InputOrderType
}

// Config returns the validated hash configuration.
func (h HMACSHA256) Config() (internal.HashConfig, error) {
	return hmacConfig("HMAC_SHA256", h.Key, h.InputOrder)
}

// HMACSHA512 represents the HMAC SHA512 hash algorithm.
//
// Key is required.
// Refer to https://firebase.google.com/docs/auth/admin/import-users#import_users_with_hmac_hashed_passwords
// for more details.
type HMACSHA512 struct {
	Key        []byte
	InputOrder InputOrderType
}

// Config returns the validated hash configuration.
func (h HMACSHA512) Config() (internal.HashConfig, error) {
	return hmacConfig("HMAC_SHA512", h.Key, h.InputOrder)
}

// MD5 represents the MD5 hash algorithm.
//
// Rounds must be between 0 and 8192.
// Refer to https://firebase.google.com/docs/auth/admin/import-users#import_users_with_md5_sha_and_pbkdf_hashed_passwords
// for more details.
type MD5 struct {
	Rounds     int
	InputOrder InputOrderType
}

// Config returns the validated hash configuration.
func (h MD5) Config() (internal.HashConfig, error) {
	return basicConfig("MD5", h.Rounds, h.InputOrder)
}

// PBKDF2SHA256 represents the PBKDF2SHA256 hash algorithm.
//
// Rounds must be between 0 and 120000.
// Refer to https://firebase.google.com/docs/auth/admin/import-users#import_users_with_md5_sha_and_pbkdf_hashed_passwords
// for more details.
type PBKDF2SHA256 struct {
	Rounds int
}

// Config returns the validated hash configuration.
func (h PBKDF2SHA256) Config() (internal.HashConfig, error) {
	return basicConfig("PBKDF2_SHA256", h.Rounds, InputOrderUnspecified)
}

// PBKDFSHA1 represents the PBKDFSHA1 hash algorithm.
//
// Rounds must be between 0 and 120000.
// Refer to https://firebase.google.com/docs/auth/admin/import-users#import_users_with_md5_sha_and_pbkdf_hashed_passwords
// for more details.
type PBKDFSHA1 struct {
	Rounds int
}

// Config returns the validated hash configuration.
func (h PBKDFSHA1) Config() (internal.HashConfig, error) {
	return basicConfig("PBKDF_SHA1", h.Rounds, InputOrderUnspecified)
}

// SHA1 represents the SHA1 hash algorithm.
//
// Rounds must be between 1 and 8192.
// Refer to https://firebase.google.com/docs/auth/admin/import-users#import_users_with_md5_sha_and_pbkdf_hashed_passwords
// for more details.
type SHA1 struct {
	Rounds     int
	InputOrder InputOrderType
}

// Config returns the validated hash configuration.
func (h SHA1) Config() (internal.HashConfig, error) {
	return basicConfig("SHA1", h.Rounds, h.InputOrder)
}

// SHA256 represents the SHA256 hash algorithm.
//
// Rounds must be between 1 and 8192.
// Refer to https://firebase.google.com/docs/auth/admin/import-users#import_users_with_md5_sha_and_pbkdf_hashed_passwords
// for more details.
type SHA256 struct {
	Rounds     int
	InputOrder InputOrderType
}

// Config returns the validated hash configuration.
func (h SHA256) Config() (internal.HashConfig, error) {
	return basicConfig("SHA256", h.Rounds, h.InputOrder)
}

// SHA512 represents the SHA512 hash algorithm.
//
// Rounds must be between 1 and 8192.
// Refer to https://firebase.google.com/docs/auth/admin/import-users#import_users_with_md5_sha_and_pbkdf_hashed_passwords
// for more details.
type SHA512 struct {
	Rounds     int
	InputOrder InputOrderType
}

// Config returns the validated hash configuration.
func (h SHA512) Config() (internal.HashConfig, error) {
	return basicConfig("SHA512", h.Rounds, h.InputOrder)
}

func hmacConfig(name string, key []byte, order InputOrderType) (internal.HashConfig, error) {
	if len(key) == 0 {
		return nil, errors.New("signer key not specified")
	}
	conf := internal.HashConfig{
		"hashAlgorithm": name,
		"signerKey":     base64.RawURLEncoding.EncodeToString(key),
	}
	if order == InputOrderSaltFirst {
		conf["passwordHashOrder"] = "SALT_AND_PASSWORD"
	} else if order == InputOrderPasswordFirst {
		conf["passwordHashOrder"] = "PASSWORD_AND_SALT"
	}
	return conf, nil
}

func basicConfig(name string, rounds int, order InputOrderType) (internal.HashConfig, error) {
	minRounds := 0
	maxRounds := 120000
	switch name {
	case "MD5":
		maxRounds = 8192
	case "SHA1", "SHA256", "SHA512":
		minRounds = 1
		maxRounds = 8192
	}
	if rounds < minRounds || maxRounds < rounds {
		return nil, fmt.Errorf("rounds must be between %d and %d", minRounds, maxRounds)
	}

	conf := internal.HashConfig{
		"hashAlgorithm": name,
		"rounds":        rounds,
	}
	if order == InputOrderSaltFirst {
		conf["passwordHashOrder"] = "SALT_AND_PASSWORD"
	} else if order == InputOrderPasswordFirst {
		conf["passwordHashOrder"] = "PASSWORD_AND_SALT"
	}
	return conf, nil
}
