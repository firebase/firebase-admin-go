// Package internal contains functionality that is only accessible from within the Admin SDK.
package internal

import (
	"golang.org/x/oauth2/google"
)

// AuthConfig represents the configuration of Firebase Auth service.
type AuthConfig struct {
	Creds     *google.DefaultCredentials
	ProjectID string
}
