package auth

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/firebase/firebase-admin-go/credentials"
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

var keys keySource = newHTTPKeySource(googleCertURL)

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

// Auth is the interface for the Firebase auth service.
//
// Auth facilitates generating custom JWT tokens for Firebase clients, and verifying ID tokens issued
// by Firebase backend services.
type Auth interface {
	// CustomToken creates a signed custom authentication token with the specified user ID. The resulting
	// JWT can be used in a Firebase client SDK to trigger an authentication flow.
	CustomToken(uid string) (string, error)

	// CustomTokenWithClaims is similar to CustomToken, but in addition to the user ID, it also encodes
	// all the key-value pairs in the provided map as claims in the resulting JWT.
	CustomTokenWithClaims(uid string, devClaims map[string]interface{}) (string, error)

	// VerifyIDToken verifies the signature	and payload of the provided ID token.
	//
	// VerifyIDToken accepts a signed JWT token string, and verifies that it is current, issued for the
	// correct Firebase project, and signed by the Google Firebase services in the cloud. It returns
	// a Token containing the decoded claims in the input JWT.
	VerifyIDToken(idToken string) (*Token, error)
}

// Signer represents an entity that can be used to sign custom JWT tokens.
//
// Credential implementations that intend to support custom token minting must implement this interface.
type Signer interface {
	ServiceAcctEmail() string
	Sign(data string) ([]byte, error)
}

// ProjectMember represents an entity that can be used to obtain a Firebase project ID.
//
// ProjectMember is used during ID token verification. The ID tokens passed to VerifyIDToken must contain the project
// ID returned by this interface for them to be considered valid. Credential implementations that intend to support ID
// token verification must implement this interface.
type ProjectMember interface {
	ProjectID() string
}

// New creates a new instance of the Firebase Auth service.
//
// This function can only be invoked from within the SDK. Client applications should access the
// the Auth service through the apps.App interface.
func New(c *internal.AppConf) Auth {
	return &authImpl{c.Cred, false}
}

type authImpl struct {
	cred    credentials.Credential
	deleted bool
}

func (a *authImpl) CustomToken(uid string) (string, error) {
	return a.CustomTokenWithClaims(uid, make(map[string]interface{}))
}

func (a *authImpl) CustomTokenWithClaims(uid string, devClaims map[string]interface{}) (string, error) {
	if a.deleted {
		return "", errors.New("parent Firebase app instance has been deleted")
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

	signer, ok := a.cred.(Signer)
	if !ok {
		return "", errors.New("must initialize Firebase App with a credential that supports token signing")
	}

	now := clk.Now().Unix()
	payload := &customToken{
		Iss:    signer.ServiceAcctEmail(),
		Sub:    signer.ServiceAcctEmail(),
		Aud:    firebaseAudience,
		UID:    uid,
		Iat:    now,
		Exp:    now + tokenExpSeconds,
		Claims: devClaims,
	}
	return encodeToken(defaultHeader(), payload, signer)
}

func (a *authImpl) VerifyIDToken(idToken string) (*Token, error) {
	if a.deleted {
		return nil, errors.New("parent Firebase app instance has been deleted")
	}
	if idToken == "" {
		return nil, fmt.Errorf("ID token must be a non-empty string")
	}

	var projectID string
	pm, ok := a.cred.(ProjectMember)
	if ok {
		projectID = pm.ProjectID()
	} else {
		projectID = os.Getenv(gcloudProject)
		if projectID == "" {
			return nil, fmt.Errorf("must initialize Firebase App with a credential that supports token "+
				"verification, or set your project ID as the %q environment variable to call "+
				"VerifyIDToken()", gcloudProject)
		}
	}

	h := &jwtHeader{}
	p := &Token{}
	if err := decodeToken(idToken, keys, h, p); err != nil {
		return nil, err
	}

	projectIDMsg := "Make sure the ID token comes from the same Firebase project as the credential used to" +
		" authenticate this SDK."
	verifyTokenMsg := "See https://firebase.google.com/docs/auth/admin/verify-id-tokens for details on how to " +
		"retrieve a valid ID token."
	issuer := issuerPrefix + projectID

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
	} else if p.Audience != projectID {
		err = fmt.Errorf("ID token has invalid 'aud' (audience) claim. Expected %q but got %q. %s %s",
			projectID, p.Audience, projectIDMsg, verifyTokenMsg)
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

func (a *authImpl) Del() {
	a.cred = nil
	a.deleted = true
}
