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

// Package internal contains functionality that is only accessible from within the Admin SDK.
package internal // import "firebase.google.com/go/internal"

import (
	"fmt"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

// FirebaseScopes is the set of OAuth2 scopes used by the Admin SDK.
var FirebaseScopes = []string{
	"https://www.googleapis.com/auth/cloud-platform",
	"https://www.googleapis.com/auth/datastore",
	"https://www.googleapis.com/auth/devstorage.full_control",
	"https://www.googleapis.com/auth/firebase",
	"https://www.googleapis.com/auth/identitytoolkit",
	"https://www.googleapis.com/auth/userinfo.email",
}

// AuthConfig represents the configuration of Firebase Auth service.
type AuthConfig struct {
	Opts      []option.ClientOption
	Creds     *google.DefaultCredentials
	ProjectID string
	Version   string
}

// InstanceIDConfig represents the configuration of Firebase Instance ID service.
type InstanceIDConfig struct {
	Opts      []option.ClientOption
	ProjectID string
}

// DatabaseConfig represents the configuration of Firebase Database service.
type DatabaseConfig struct {
	Opts         []option.ClientOption
	URL          string
	Version      string
	AuthOverride map[string]interface{}
}

// StorageConfig represents the configuration of Google Cloud Storage service.
type StorageConfig struct {
	Opts   []option.ClientOption
	Bucket string
}

// MessagingConfig represents the configuration of Firebase Cloud Messaging service.
type MessagingConfig struct {
	Opts      []option.ClientOption
	ProjectID string
	Version   string
}

// FirebaseError is an error type containing an error code string.
type FirebaseError struct {
	Code   string
	String string
}

func (fe *FirebaseError) Error() string {
	return fe.String
}

// HasErrorCode checks if the given error contain a specific error code.
func HasErrorCode(err error, code string) bool {
	fe, ok := err.(*FirebaseError)
	return ok && fe.Code == code
}

// Error creates a new FirebaseError from the specified error code and message.
func Error(code string, msg string) *FirebaseError {
	return &FirebaseError{
		Code:   code,
		String: msg,
	}
}

// Errorf creates a new FirebaseError from the specified error code and message.
func Errorf(code string, msg string, args ...interface{}) *FirebaseError {
	return Error(code, fmt.Sprintf(msg, args...))
}

// MockTokenSource is a TokenSource implementation that can be used for testing.
type MockTokenSource struct {
	AccessToken string
}

// Token returns the test token associated with the TokenSource.
func (ts *MockTokenSource) Token() (*oauth2.Token, error) {
	return &oauth2.Token{AccessToken: ts.AccessToken}, nil
}
