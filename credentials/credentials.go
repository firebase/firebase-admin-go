// Package credentials provides functions for creating authentication credentials, which can be used to initialize the
// Firebase SDK.
package credentials

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/jwt"
)

var firebaseScopes = []string{
	"https://www.googleapis.com/auth/firebase",
	"https://www.googleapis.com/auth/userinfo.email",
}

// Credential represents an authentication credential that can be used to initialize the Admin SDK.
//
// Credential provides the Admin SDK with OAuth2 access tokens to authenticate with various Firebase cloud services.
// The Credential implementations are not required to cache the OAuth2 tokens, but they are free to do so.
type Credential interface {
	// AccessToken fetches a valid, unexpired OAuth2 access token.
	//
	// AccessToken returns an access token string along with its expiry time, which allows higher-level code to
	// implement token caching. The returned token should be valid for authenticating against various Google and
	// Firebase services that the SDK needs to interact with. Generally, user applications do not have to call
	// AccessToken directly. The Admin SDK should manage calling AccessToken on behalf of user applications.
	AccessToken(ctx context.Context) (string, time.Time, error)
}

type certificate struct {
	Config *jwt.Config
	PK     *rsa.PrivateKey
	ProjID string
}

func (c *certificate) AccessToken(ctx context.Context) (string, time.Time, error) {
	source := c.Config.TokenSource(ctx)
	token, err := source.Token()
	if err != nil {
		return "", time.Time{}, err
	}
	return token.AccessToken, token.Expiry, nil
}

func (c *certificate) ServiceAcctEmail() string {
	return c.Config.Email
}

func (c *certificate) Sign(data string) ([]byte, error) {
	h := sha256.New()
	h.Write([]byte(data))
	return rsa.SignPKCS1v15(rand.Reader, c.PK, crypto.SHA256, h.Sum(nil))
}

func (c *certificate) ProjectID() string {
	return c.ProjID
}

// NewCert creates a new Credential from the provided service account certificate JSON.
//
// Service account certificate JSON files (also known as service account private keys) can be downloaded from the
// "Settings" tab of a Firebase project in the Firebase console (https://console.firebase.google.com). See
// https://firebase.google.com/docs/admin/setup for code samples and detailed documentation.
//
// NewCert consumes all the content available in the provided service account certificate Reader. It is safe to close
// the Reader once NewCert has returned.
func NewCert(r io.Reader) (Credential, error) {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	config, err := google.JWTConfigFromJSON(b, firebaseScopes...)
	if err != nil {
		return nil, err
	}

	if config.Email == "" {
		return nil, errors.New("'client_email' field not available")
	} else if config.TokenURL == "" {
		return nil, errors.New("'token_uri' field not available")
	} else if config.PrivateKey == nil {
		return nil, errors.New("'private_key' field not available")
	} else if config.PrivateKeyID == "" {
		return nil, errors.New("'private_key_id' field not available")
	}

	s := &struct {
		ProjectID string `json:"project_id"`
	}{}
	if err = json.Unmarshal(b, s); err != nil {
		return nil, err
	} else if s.ProjectID == "" {
		return nil, errors.New("'project_id' field not available")
	}

	pk, err := parseKey(config.PrivateKey)
	if err != nil {
		return nil, err
	}
	return &certificate{Config: config, PK: pk, ProjID: s.ProjectID}, nil
}

type refreshToken struct {
	Config *oauth2.Config
	Token  *oauth2.Token
}

func (c *refreshToken) AccessToken(ctx context.Context) (string, time.Time, error) {
	source := c.Config.TokenSource(ctx, c.Token)
	token, err := source.Token()
	if err != nil {
		return "", time.Time{}, err
	}
	return token.AccessToken, token.Expiry, nil
}

// NewRefreshToken creates a new Credential from the provided refresh token JSON.
//
// The refresh token JSON must contain refresh_token, client_id and client_secret fields in addition to a type
// field set to the value "authorized_user". These files are usually created and managed by the Google Cloud SDK.
func NewRefreshToken(r io.Reader) (Credential, error) {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	rt := &struct {
		Type         string `json:"type"`
		ClientSecret string `json:"client_secret"`
		ClientID     string `json:"client_id"`
		RefreshToken string `json:"refresh_token"`
	}{}
	if err := json.Unmarshal(b, rt); err != nil {
		return nil, err
	}
	if rt.Type != "authorized_user" {
		return nil, fmt.Errorf("'type' field is '%s' (expected 'authorized_user')", rt.Type)
	} else if rt.ClientID == "" {
		return nil, fmt.Errorf("'client_id' field not available")
	} else if rt.ClientSecret == "" {
		return nil, fmt.Errorf("'client_secret' field not available")
	} else if rt.RefreshToken == "" {
		return nil, fmt.Errorf("'refresh_token' field not available")
	}
	config := &oauth2.Config{
		ClientID:     rt.ClientID,
		ClientSecret: rt.ClientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       firebaseScopes,
	}
	token := &oauth2.Token{
		RefreshToken: rt.RefreshToken,
	}
	return &refreshToken{Config: config, Token: token}, nil
}

type appDefault struct {
	Credential *google.DefaultCredentials
}

func (c *appDefault) AccessToken(ctx context.Context) (string, time.Time, error) {
	source := c.Credential.TokenSource
	token, err := source.Token()
	if err != nil {
		return "", time.Time{}, err
	}
	return token.AccessToken, token.Expiry, nil
}

// NewAppDefault creates a new Credential based on the runtime environment.
//
// NewAppDefault inspects the runtime environment to fetch a valid set of authentication credentials. This is
// particularly useful when deployed in a managed cloud environment such as Google App Engine or Google Compute Engine.
// Refer https://developers.google.com/identity/protocols/application-default-credentials for more details on how
// application default credentials work.
func NewAppDefault(ctx context.Context) (Credential, error) {
	cred, err := google.FindDefaultCredentials(ctx, firebaseScopes...)
	if err != nil {
		return nil, err
	}
	return &appDefault{Credential: cred}, nil
}

func parseKey(key []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(key)
	if block != nil {
		key = block.Bytes
	}
	parsedKey, err := x509.ParsePKCS8PrivateKey(key)
	if err != nil {
		parsedKey, err = x509.ParsePKCS1PrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("private key should be a PEM or plain PKSC1 or PKCS8; parse error: %v", err)
		}
	}
	parsed, ok := parsedKey.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not an RSA key")
	}
	return parsed, nil
}
