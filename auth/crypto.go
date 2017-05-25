package auth

import (
	"errors"
)

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

// decodeToken decodes a token and verifies the signature. Requires that the
// token is signed using the RS256 algorithm.
func (a *Auth) decodeToken(token string) (*Token, error) {
	return nil, errors.New("Not yet implemented")
}
