// Package appcheck provides functionality for verifying AppCheck tokens.
package appcheck

import (
	"context"
	"errors"
	"log"
	"strings"

	"github.com/MicahParks/keyfunc"
	"github.com/golang-jwt/jwt/v4"

	"firebase.google.com/go/v4/auth"
	"firebase.google.com/go/v4/internal"
)

const (
	AppCheckIssuer = "https://firebaseappcheck.googleapis.com/"
	JWKSUrl        = "https://firebaseappcheck.googleapis.com/v1beta/jwks"
)

type VerifiedToken struct {
	AppID string

	Token auth.Token
}

type Client struct {
	projectID string

	jwks *keyfunc.JWKS
}

// NewClient creates a new AppCheck client.
func NewClient(ctx context.Context, conf *internal.AppCheckConfig) (*Client, error) {
	// TODO: Consider passing a context.Context or http.Client.
	jwks, err := keyfunc.Get(JWKSUrl, keyfunc.Options{})
	if err != nil {
		return nil, err
	}

	return &Client{
		projectID: conf.ProjectID,
		jwks:      jwks,
	}, nil
}

// VerifyToken verifies the given AppCheck token.
// It returns a VerifiedToken if valid and an error if invalid.
func (c *Client) VerifyToken(token string) (*VerifiedToken, error) {
	// Reference for checks:
	// https://github.com/firebase/firebase-admin-node/blob/master/src/app-check/token-verifier.ts#L106

	decodedToken, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		if t.Header["alg"] != "RS256" {
			return nil, errors.New("app check token has incorrect algorithm")
		}
		return c.jwks.Keyfunc(t)
	})
	if err != nil {
		return nil, err
	}

	claims, ok := decodedToken.Claims.(jwt.MapClaims)
	log.Printf("claims: %+v", claims)
	if !ok {
		return nil, errors.New("app check token has incorrect claims")
	}

	rawAud := claims["aud"].([]interface{})
	aud := []string{}
	for _, v := range rawAud {
		aud = append(aud, v.(string))
	}
	if !contains(aud, "projects/"+c.projectID) {
		return nil, errors.New("app check token has incorrect audience")
	}

	if !strings.HasPrefix(claims["iss"].(string), AppCheckIssuer) {
		return nil, errors.New("app check token has incorrect issuer")
	}

	if _, ok := claims["sub"].(string); !ok {
		return nil, errors.New("app check token has no subject")
	}

	if val := claims["sub"].(string); val == "" {
		return nil, errors.New("app check token has empty subject")
	}

	return &VerifiedToken{
		AppID: decodedToken.Claims.(jwt.MapClaims)["sub"].(string),
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
