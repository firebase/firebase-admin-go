package auth

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/firebase/firebase-admin-go/internal"
)

const (
	issuerPrefix = "https://securetoken.google.com/"
)

var (
	timeNow = time.Now
)

// Auth provides methods for creating and validating Firebase Auth tokens.
type Auth struct {
	hc  *http.Client
	kc  *keyCache
	pid string
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
		kc: &keyCache{
			hc: c.Client,
		},
		pid: c.ProjectID,
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
	if idToken == "" {
		return nil, fmt.Errorf("ID token must be a non-empty string")
	}
	if a.pid == "" {
		return nil, fmt.Errorf("unkown project ID")
	}

	issuer := issuerPrefix + a.pid

	t, err := a.decodeToken(idToken)
	if err != nil {
		return nil, err
	}

	projectIDMsg := "Make sure the ID token comes from the same Firebase project as the credential used to" +
		" authenticate this SDK."
	verifyTokenMsg := "See https://firebase.google.com/docs/auth/admin/verify-id-tokens for details on how to " +
		"retrieve a valid ID token."

	switch {
	case t.Audience != a.pid:
		return nil, fmt.Errorf("ID token has invalid 'aud' (audience) claim. Expected %q but got %q. %s %s",
			a.pid, t.Audience, projectIDMsg, verifyTokenMsg)
	case t.Issuer != issuer:
		return nil, fmt.Errorf("ID token has invalid 'iss' (issuer) claim. Expected %q but got %q. %s %s",
			issuer, t.Issuer, projectIDMsg, verifyTokenMsg)
	case t.IssuedAt > timeNow().Unix():
		return nil, fmt.Errorf("ID token issued at future timestamp: %d", t.IssuedAt)
	case t.Expires < timeNow().Unix():
		return nil, fmt.Errorf("ID token has expired. Expired at: %d", t.Expires)
	case t.Subject == "":
		return nil, fmt.Errorf("ID token has empty 'sub' (subject) claim. %s", verifyTokenMsg)
	case len(t.Subject) > 128:
		return nil, fmt.Errorf("ID token has a 'sub' (subject) claim longer than 128 characters. %s", verifyTokenMsg)
	}

	t.UID = t.Subject
	return t, nil
}
