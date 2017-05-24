package auth

import (
	"errors"
	"net/http"

	"github.com/firebase/firebase-admin-go/internal"
)

// Auth provides methods for creating and validating Firebase Auth tokens.
type Auth struct {
	hc *http.Client
}

// Token represents a decoded Firebase ID token.
//
// Token provides typed accessors to the common JWT fields such as Audience
// (aud) and Expiry (exp). Additionally it provides a UID field, which indicates
// the user ID of the account to which this token belongs. Any additional JWT
// claims can be accessed via the Claims map of Token.
type Token struct {
	Issuer   string                 `json:"iss"`
	Audience string                 `json:"aud"`
	Expires  int64                  `json:"exp"`
	IssuedAt int64                  `json:"iat"`
	Subject  string                 `json:"sub,omitempty"`
	UID      string                 `json:"uid,omitempty"`
	Claims   map[string]interface{} `json:"-"`
}

// New creates a new Firebase Auth client.
func New(c *internal.AuthConfig) *Auth {
	return &Auth{
		hc: c.Client,
	}
}

// CustomToken creates a signed custom authentication token with the specified
// user ID. The resulting JWT can be used in a Firebase client SDK to trigger an
// authentication flow.
func (a *Auth) CustomToken(uid string) (string, error) {
	return "", errors.New("Not yet implemented")
}

// CustomTokenWithClaims is similar to CustomToken, but in addition to the user ID, it also encodes
// all the key-value pairs in the provided map as claims in the resulting JWT.
func (a *Auth) CustomTokenWithClaims(uid string, claims map[string]interface{}) (string, error) {
	return "", errors.New("Not yet implemented")
}

// VerifyIDToken verifies the signature	and payload of the provided ID token.
//
// VerifyIDToken accepts a signed JWT token string, and verifies that it is current, issued for the
// correct Firebase project, and signed by the Google Firebase services in the cloud. It returns
// a Token containing the decoded claims in the input JWT.
func (a *Auth) VerifyIDToken(idToken string) (*Token, error) {
	return nil, errors.New("Not yet implemented")
}
