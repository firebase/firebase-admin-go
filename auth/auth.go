package auth

import (
	"fmt"
	"net/http"
	"time"

	"github.com/firebase/firebase-admin-go/internal"
)

const (
	firebaseAudience    = "https://identitytoolkit.googleapis.com/google.identity.identitytoolkit.v1.IdentityToolkit"
	issuerPrefix        = "https://securetoken.google.com/"
	customTokenDuration = 1 * time.Hour
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
	return a.CustomTokenWithClaims(uid, nil)
}

// CustomTokenWithClaims is similar to CustomToken, but in addition to the user ID, it also encodes
// all the key-value pairs in the provided map as claims in the resulting JWT.
func (a *Auth) CustomTokenWithClaims(uid string, claims map[string]interface{}) (string, error) {
	if n := len(uid); n == 0 || n > 128 {
		return "", fmt.Errorf("creating token: invalid UID: %q", uid)
	}
	now := timeNow()
	return a.encodeToken(rawClaims{
		"aud":    firebaseAudience,
		"claims": claims,
		"exp":    now.Add(customTokenDuration).Unix(),
		"iat":    now.Unix(),
		"uid":    uid,
	})
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
	}

	t.UID = t.Subject
	return t, nil
}
