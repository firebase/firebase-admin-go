// Package appcheck provides functionality for verifying App Check tokens.
package appcheck

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc"
	"github.com/golang-jwt/jwt/v4"

	"firebase.google.com/go/v4/internal"
)

const (
	AppCheckIssuer = "https://firebaseappcheck.googleapis.com/"
	JWKSUrl        = "https://firebaseappcheck.googleapis.com/v1beta/jwks"
)

var (
	ErrIncorrectAlgorithm = errors.New("token has incorrect algorithm")
	ErrTokenType          = errors.New("token has incorrect type")
	ErrTokenClaims        = errors.New("token has incorrect claims")
	ErrTokenAudience      = errors.New("token has incorrect audience")
	ErrTokenIssuer        = errors.New("token has incorrect issuer")
	ErrTokenSubject       = errors.New("token has empty or missing subject")
)

type VerifiedToken struct {
	Iss   string
	Sub   string
	Aud   []string
	Exp   time.Time
	Iat   time.Time
	AppID string
}

type Client struct {
	projectID string

	jwks *keyfunc.JWKS
}

// NewClient creates a new App Check client.
func NewClient(ctx context.Context, conf *internal.AppCheckConfig) (*Client, error) {
	// TODO: Add support for overriding the HTTP client using the App one.
	jwks, err := keyfunc.Get(conf.JWKSUrl, keyfunc.Options{
		Ctx: ctx,
	})
	if err != nil {
		return nil, err
	}

	return &Client{
		projectID: conf.ProjectID,
		jwks:      jwks,
	}, nil
}

// VerifyToken verifies the given App Check token.
// It returns a VerifiedToken if valid and an error if invalid.
func (c *Client) VerifyToken(token string) (*VerifiedToken, error) {
	// References for checks:
	// https://firebase.googleblog.com/2021/10/protecting-backends-with-app-check.html
	// https://github.com/firebase/firebase-admin-node/blob/master/src/app-check/token-verifier.ts#L106

	// The standard JWT parser also validates the expiration of the token
	// so we do not need dedicated code for that.
	decodedToken, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		if t.Header["alg"] != "RS256" {
			return nil, ErrIncorrectAlgorithm
		}
		if t.Header["typ"] != "JWT" {
			return nil, ErrTokenType
		}
		return c.jwks.Keyfunc(t)
	})
	if err != nil {
		return nil, err
	}

	claims, ok := decodedToken.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrTokenClaims
	}

	rawAud := claims["aud"].([]interface{})
	aud := []string{}
	for _, v := range rawAud {
		aud = append(aud, v.(string))
	}

	if !contains(aud, "projects/"+c.projectID) {
		return nil, ErrTokenAudience
	}

	// We check the prefix to make sure this token was issued
	// by the Firebase App Check service, but we do not check the
	// Project Number suffix because the Golang SDK only has project ID.
	//
	// This is consistent with the Firebase Admin Node SDK.
	if !strings.HasPrefix(claims["iss"].(string), AppCheckIssuer) {
		return nil, ErrTokenIssuer
	}

	if val, ok := claims["sub"].(string); !ok || val == "" {
		return nil, ErrTokenSubject
	}

	return &VerifiedToken{
		Iss:   claims["iss"].(string),
		Sub:   claims["sub"].(string),
		Aud:   aud,
		Exp:   time.Unix(int64(claims["exp"].(float64)), 0),
		Iat:   time.Unix(int64(claims["iat"].(float64)), 0),
		AppID: claims["sub"].(string),
	}, nil
}

func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}
