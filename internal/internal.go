// Package internal contains functionality that is only accessible from within the Admin SDK.
package internal

import (
	"golang.org/x/oauth2/jwt"
)

// AuthConfig represents the configuration of Firebase Auth service.
type AuthConfig struct {
	Config    *jwt.Config
	ProjectID string
}
