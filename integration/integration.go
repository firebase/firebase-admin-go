package integration

import (
	"context"
	"io/ioutil"

	firebase "github.com/firebase/firebase-admin-go"
	"google.golang.org/api/option"
)

// NewAppForTest creates a new App instance that can be used in integration tests.
func NewAppForTest() (*firebase.App, error) {
	opt := option.WithCredentialsFile("../testdata/integration_cert.json")
	return firebase.NewApp(context.Background(), nil, opt)
}

// APIKey fetches a Firebase API key that can be used in integration tests.
func APIKey() (string, error) {
	b, err := ioutil.ReadFile("../testdata/integration_apikey.txt")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
