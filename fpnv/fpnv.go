// Copyright 2026 Google LLC All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package fpnv provides functionality for Firebase Phone Number Verification (FPNV) tokens.
package fpnv

import (
    "context"
    "errors"
    "slices"
    "strings"
    "time"

    "github.com/MicahParks/keyfunc"
    "github.com/golang-jwt/jwt/v4"

    "firebase.google.com/go/v4/internal"
)

const (
    fpnvJWKSURL = "https://fpnv.googleapis.com/v1beta/jwks"
    fpnvIssuer  = "https://fpnv.googleapis.com/projects/"
    algorithm   = "ES256"
    headerTyp   = "JWT"
)

var (
    // ErrTokenHeaderKid is returned when the token has no 'kid' claim.
    ErrTokenHeaderKid = errors.New("FPNV token has no 'kid' claim")
    // ErrIncorrectAlgorithm is returned when the token is signed with a non-ES256 algorithm.
    ErrIncorrectAlgorithm = errors.New("FPNV token has incorrect algorithm")
    // ErrTokenType is returned when the token is not a JWT.
    ErrTokenType = errors.New("FPNV token has incorrect type")
    // ErrTokenClaims is returned when the token claims cannot be decoded.
    ErrTokenClaims = errors.New("FPNV token has incorrect claims")
    // ErrTokenAudience is returned when the token audience does not match the current project.
    ErrTokenAudience = errors.New("FPNV token has incorrect audience")
    // ErrTokenIssuer is returned when the token issuer does not match FPNV service.
    ErrTokenIssuer = errors.New("FPNV token has incorrect issuer")
    // ErrTokenSubject is returned when the token subject is empty or missing.
    ErrTokenSubject = errors.New("FPNV token has empty or missing subject")
)

// DecodedFpnvToken represents a verified FPNV token.
//
// DecodedFpnvToken provides typed accessors to the common JWT fields such as Audience (aud)
// and ExpiresAt (exp). Additionally, it provides an PhoneNumber field,
// which is alias for Subject (sub).
// Any additional JWT claims can be accessed via the Claims map of DecodedFpnvToken.
type DecodedFpnvToken struct {
    Issuer      string
    Subject     string
    Audience    []string
    ExpiresAt   time.Time
    IssuedAt    time.Time
    PhoneNumber string
    Claims      map[string]interface{}
}

// Client is the client for the Firebase Phone Number Verification service.
type Client struct {
    projectID string
    jwks      *keyfunc.JWKS
}

// NewClient creates a new instance of the Firebase Phone Number Verification Client.
//
// This function can only be invoked from within the SDK. Client applications should access the
// FPNV service through firebase.App.
func NewClient(ctx context.Context, conf *internal.FpnvConfig) (*Client, error) {
    jwks, err := keyfunc.Get(fpnvJWKSURL, keyfunc.Options{
       Ctx:             ctx,
       RefreshInterval: 10 * time.Minute,
    })
    if err != nil {
       return nil, err
    }

    return &Client{
       projectID: conf.ProjectID,
       jwks:      jwks,
    }, nil
}

// VerifyToken verifies the given Firebase Phone Number Verification (FPNV) token.
//
// VerifyToken considers a Firebase Phone Number Verification token string to be valid
// if all the following conditions are met:
//   - The token string is a valid ES256 JWT.
//   - The JWT contains valid issuer (iss) and audience (aud) claims that match the issuerPrefix
//     and projectID of the tokenVerifier.
//   - The JWT is not expired, and it has been issued some time in the past.
//
// If any of the above conditions are not met, an error is returned.
// Otherwise, a pointer to a decoded FPNV token is returned.
func (c *Client) VerifyToken(token string) (*DecodedFpnvToken, error) {
    // The standard JWT parser also validates the expiration of the token
    // so we do not need dedicated code for that.

    // Header part
    decodedToken, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
       if t.Header["kid"] == nil {
          return nil, ErrTokenHeaderKid
       }
       if t.Header["alg"] != algorithm {
          return nil, ErrIncorrectAlgorithm
       }
       if t.Header["typ"] != headerTyp {
          return nil, ErrTokenType
       }
       return c.jwks.Keyfunc(t)
    })
    if err != nil {
       return nil, err
    }

    // Payload part
    claims, ok := decodedToken.Claims.(jwt.MapClaims)
    if !ok {
       return nil, ErrTokenClaims
    }

    rawAud := claims["aud"].([]interface{})
    var aud []string
    for _, v := range rawAud {
       aud = append(aud, v.(string))
    }

    if !slices.Contains(aud, fpnvIssuer+c.projectID) {
       return nil, ErrTokenAudience
    }

    // We check the prefix to make sure this token was issued
    // by the Firebase Phone Number Verification service, but we do not check the
    // Project Number suffix because the Golang SDK only has project ID.
    //
    // This is consistent with the Firebase Admin Node SDK.
    if !strings.HasPrefix(claims["iss"].(string), fpnvIssuer) {
       return nil, ErrTokenIssuer
    }

    if val, ok := claims["sub"].(string); !ok || val == "" {
       return nil, ErrTokenSubject
    }

    decodedFpnvToken := DecodedFpnvToken{
       Issuer:      claims["iss"].(string),
       Subject:     claims["sub"].(string),
       Audience:    aud,
       ExpiresAt:   time.Unix(int64(claims["exp"].(float64)), 0),
       IssuedAt:    time.Unix(int64(claims["iat"].(float64)), 0),
       PhoneNumber: claims["sub"].(string),
    }

    // Remove all the claims we've already parsed.
    for _, usedClaim := range []string{"iss", "sub", "aud", "exp", "iat", "sub"} {
       delete(claims, usedClaim)
    }
    decodedFpnvToken.Claims = claims

    return &decodedFpnvToken, nil
}
