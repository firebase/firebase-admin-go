package auth

import (
	"errors"
)

// decodeToken decodes a token and verifies the signature. Requires that the
// token is signed using the RS256 algorithm.
func (a *Auth) decodeToken(token string) (*Token, error) {
	return nil, errors.New("Not yet implemented")
}
