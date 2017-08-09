// Package integration contains utilities for running integration tests.
package integration

import (
	"io/ioutil"

	"golang.org/x/net/context"

	firebase "github.com/firebase/firebase-admin-go"
	"google.golang.org/api/option"
)

const certPath = "../testdata/integration_cert.json"
const apiKeyPath = "../testdata/integration_apikey.txt"

// NewAppForTest creates a new App instance for integration tests.
//
// NewAppForTest looks for a service account JSON file named integration_cert.json
// in the testdata directory. This file is used to initialize the newly created
// App instance.
func NewAppForTest(ctx context.Context) (*firebase.App, error) {
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
	return string(b), nil
}
