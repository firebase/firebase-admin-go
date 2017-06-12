package auth

import (
	"encoding/pem"
	"errors"
	"fmt"
	"strings"

	"crypto/rsa"
	"crypto/x509"

	"github.com/firebase/firebase-admin-go/internal"
)

const firebaseAudience = "https://identitytoolkit.googleapis.com/google.identity.identitytoolkit.v1.IdentityToolkit"
const googleCertURL = "https://www.googleapis.com/robot/v1/metadata/x509/securetoken@system.gserviceaccount.com"
const issuerPrefix = "https://securetoken.google.com/"
const gcloudProject = "GCLOUD_PROJECT"
const tokenExpSeconds = 3600

var reservedClaims = []string{
	"acr", "amr", "at_hash", "aud", "auth_time", "azp", "cnf", "c_hash",
	"exp", "firebase", "iat", "iss", "jti", "nbf", "nonce", "sub",
}

var clk clock = &systemClock{}

// Token represents a decoded Firebase ID token.
//
// Token provides typed accessors to the common JWT fields such as Audience (aud) and Expiry (exp).
// Additionally it provides a UID field, which indicates the user ID of the account to which this token
// belongs. Any additional JWT claims can be accessed via the Claims map of Token.
type Token struct {
	Issuer   string                 `json:"iss"`
	Audience string                 `json:"aud"`
	Expires  int64                  `json:"exp"`
	IssuedAt int64                  `json:"iat"`
	Subject  string                 `json:"sub,omitempty"`
	UID      string                 `json:"uid,omitempty"`
	Claims   map[string]interface{} `json:"-"`
}

// Client is the interface for the Firebase auth service.
//
// Client facilitates generating custom JWT tokens for Firebase clients, and verifying ID tokens issued
// by Firebase backend services.
type Client struct {
	ks        keySource
	projectID string
	email     string
	pk        *rsa.PrivateKey
}

// NewClient creates a new instance of the Firebase Auth Client.
//
// This function can only be invoked from within the SDK. Client applications should access the
// the Auth service through admin.App.
func NewClient(c *internal.AuthConfig) (*Client, error) {
	client := &Client{ks: newHTTPKeySource(googleCertURL), projectID: c.ProjectID}
	if c.Config != nil {
		client.email = c.Config.Email
		pk, err := parseKey(c.Config.PrivateKey)
		if err != nil {
			return nil, err
		}
		client.pk = pk
	}
	return client, nil
}

// CustomToken creates a signed custom authentication token with the specified user ID. The resulting
// JWT can be used in a Firebase client SDK to trigger an authentication flow.
func (c *Client) CustomToken(uid string) (string, error) {
	return c.CustomTokenWithClaims(uid, make(map[string]interface{}))
}

// CustomTokenWithClaims is similar to CustomToken, but in addition to the user ID, it also encodes
// all the key-value pairs in the provided map as claims in the resulting JWT.
func (c *Client) CustomTokenWithClaims(uid string, devClaims map[string]interface{}) (string, error) {
	if c.email == "" {
		return "", errors.New("service account email not available")
	}
	if c.pk == nil {
		return "", errors.New("private key not available")
	}

	if len(uid) == 0 || len(uid) > 128 {
		return "", errors.New("uid must be non-empty, and not longer than 128 characters")
	}

	var disallowed []string
	for _, k := range reservedClaims {
		if _, contains := devClaims[k]; contains {
			disallowed = append(disallowed, k)
		}
	}
	if len(disallowed) == 1 {
		return "", fmt.Errorf("developer claim %q is reserved and cannot be specified", disallowed[0])
	} else if len(disallowed) > 1 {
		return "", fmt.Errorf("developer claims %q are reserved and cannot be specified", strings.Join(disallowed, ", "))
	}

	now := clk.Now().Unix()
	payload := &customToken{
		Iss:    c.email,
		Sub:    c.email,
		Aud:    firebaseAudience,
		UID:    uid,
		Iat:    now,
		Exp:    now + tokenExpSeconds,
		Claims: devClaims,
	}
	return encodeToken(defaultHeader(), payload, c.pk)
}

// VerifyIDToken verifies the signature	and payload of the provided ID token.
//
// VerifyIDToken accepts a signed JWT token string, and verifies that it is current, issued for the
// correct Firebase project, and signed by the Google Firebase services in the cloud. It returns
// a Token containing the decoded claims in the input JWT.
func (c *Client) VerifyIDToken(idToken string) (*Token, error) {
	if c.projectID == "" {
		return nil, errors.New("project id not available")
	}
	if idToken == "" {
		return nil, fmt.Errorf("ID token must be a non-empty string")
	}

	h := &jwtHeader{}
	p := &Token{}
	if err := decodeToken(idToken, c.ks, h, p); err != nil {
		return nil, err
	}

	projectIDMsg := "Make sure the ID token comes from the same Firebase project as the credential used to" +
		" authenticate this SDK."
	verifyTokenMsg := "See https://firebase.google.com/docs/auth/admin/verify-id-tokens for details on how to " +
		"retrieve a valid ID token."
	issuer := issuerPrefix + c.projectID

	var err error
	if h.KeyID == "" {
		if p.Audience == firebaseAudience {
			err = fmt.Errorf("VerifyIDToken() expects an ID token, but was given a custom token")
		} else {
			err = fmt.Errorf("ID token has no 'kid' header")
		}
	} else if h.Algorithm != "RS256" {
		err = fmt.Errorf("ID token has invalid incorrect algorithm. Expected 'RS256' but got %q. %s",
			h.Algorithm, verifyTokenMsg)
	} else if p.Audience != c.projectID {
		err = fmt.Errorf("ID token has invalid 'aud' (audience) claim. Expected %q but got %q. %s %s",
			c.projectID, p.Audience, projectIDMsg, verifyTokenMsg)
	} else if p.Issuer != issuer {
		err = fmt.Errorf("ID token has invalid 'iss' (issuer) claim. Expected %q but got %q. %s %s",
			issuer, p.Issuer, projectIDMsg, verifyTokenMsg)
	} else if p.IssuedAt > clk.Now().Unix() {
		err = fmt.Errorf("ID token issued at future timestamp: %d", p.IssuedAt)
	} else if p.Expires < clk.Now().Unix() {
		err = fmt.Errorf("ID token has expired. Expired at: %d", p.Expires)
	} else if p.Subject == "" {
		err = fmt.Errorf("ID token has empty 'sub' (subject) claim. %s", verifyTokenMsg)
	} else if len(p.Subject) > 128 {
		err = fmt.Errorf("ID token has a 'sub' (subject) claim longer than 128 characters. %s", verifyTokenMsg)
	}

	if err != nil {
		return nil, err
	}
	p.UID = p.Subject
	return p, nil
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
