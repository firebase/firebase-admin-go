// Package internal contains functionality that is only accessible from within the Admin SDK.
package internal

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"golang.org/x/oauth2/google"
)

// AuthConfig contains all the configuration needed to initialize an auth
// client.
type AuthConfig struct {
	Client    *http.Client
	Creds     *google.DefaultCredentials
	ProjectID string
}

// DatabaseConfig contains all the configuration needed to initialize a database
// client.
type DatabaseConfig struct {
	Client *http.Client
	URL    *url.URL
}

// ParseKey converts the contents of a private key file to an *rsa.PrivateKey.
func ParseKey(key string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(key))
	if block == nil {
		return nil, fmt.Errorf("no private key data found in: %v", key)
	}
	k := block.Bytes
	parsedKey, err := x509.ParsePKCS8PrivateKey(k)
	if err != nil {
		parsedKey, err = x509.ParsePKCS1PrivateKey(k)
		if err != nil {
			return nil, fmt.Errorf("private key should be a PEM or plain PKSC1 or PKCS8; parse error: %v", err)
		}
	}
	parsed, ok := parsedKey.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key is invalid")
	}
	return parsed, nil
}
