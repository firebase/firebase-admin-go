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

// Package internal contains utilities for running integration tests.
package internal

import (
	"io/ioutil"
	"strings"

	"golang.org/x/net/context"

	firebase "github.com/firebase/firebase-admin-go"
	"google.golang.org/api/option"
)

const certPath = "../testdata/integration_cert.json"
const apiKeyPath = "../testdata/integration_apikey.txt"

// NewTestApp creates a new App instance for integration tests.
//
// NewTestApp looks for a service account JSON file named integration_cert.json
// in the testdata directory. This file is used to initialize the newly created
// App instance.
func NewTestApp(ctx context.Context) (*firebase.App, error) {
	return firebase.NewApp(ctx, nil, option.WithCredentialsFile(certPath))
}

// APIKey fetches a Firebase API key for integration tests.
//
// APIKey reads the API key string from a file named integration_apikey.txt
// in the testdata directory.
func APIKey() (string, error) {
	b, err := ioutil.ReadFile(apiKeyPath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}
