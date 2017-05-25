// Package internal contains functionality that is only accessible from within the Admin SDK.
package internal

import (
	"net/http"
	"net/url"
)

// AuthConfig contains all the configuration needed to initialize an auth
// client.
type AuthConfig struct {
	Client    *http.Client
	ProjectID string
}

// DatabaseConfig contains all the configuration needed to initialize a database
// client.
type DatabaseConfig struct {
	Client *http.Client
	URL    *url.URL
}
