package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/dgrijalva/jwt-go"
)

var (
	// ErrTokenInvalid is returned when a token is invalid.
	ErrTokenInvalid = errors.New("invalid token")

	reservedClaims = []string{"aud", "exp", "iat", "iss", "nbf", "sub"}
)

// Token represents a decoded Firebase ID token.
//
// Token provides typed accessors to the common JWT fields such as Audience
// (aud) and Expiry (exp). Additionally it provides a UID field, which indicates
// the user ID of the account to which this token belongs. Any additional JWT
// claims can be accessed via the Claims map of Token.
type Token struct {
	Audience  string                 `json:"aud"`
	Claims    map[string]interface{} `json:"-"`
	ExpiresAt int64                  `json:"exp"`
	IssuedAt  int64                  `json:"iat"`
	Issuer    string                 `json:"iss"`
	NotBefore int64                  `json:"nbf"`
	Subject   string                 `json:"sub"`
	UID       string                 `json:"-"`
}

// decodeToken decodes a token and verifies the signature. Requires that the
// token is signed using the RS256 algorithm.
func (a *Auth) decodeToken(tokenString string) (*Token, error) {
	rc := rawClaims{}
	token, err := jwt.ParseWithClaims(tokenString, &rc, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("parsing token: invalid signing method: %v", token.Header["alg"])
		}
		return a.kc.get(token.Header["kid"].(string))
	})

	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, ErrTokenInvalid
	}

	uid := rc["sub"].(string)
	t := &Token{
		ExpiresAt: int64(rc["exp"].(float64)),
		IssuedAt:  int64(rc["iat"].(float64)),
		Subject:   uid,
		UID:       uid,
		Claims:    rc,
	}
	if aud, ok := rc["aud"].(string); ok {
		t.Audience = aud
	}
	if iss, ok := rc["iss"].(string); ok {
		t.Issuer = iss
	}
	if nbf, ok := rc["nbf"].(float64); ok {
		t.NotBefore = int64(nbf)
	}
	for _, c := range reservedClaims {
		delete(rc, c)
	}
	return t, nil
}

type rawClaims map[string]interface{}

func (c rawClaims) Valid() error {
	now := timeNow()
	if iat, ok := c["iat"].(float64); !ok || time.Unix(int64(iat), 0).Sub(now) > 0 {
		return fmt.Errorf("parsing token: unexpected iat: %v", c["iat"])
	}
	if nbf, ok := c["nbf"].(float64); ok && time.Unix(int64(nbf), 0).Sub(now) < 0 {
		return fmt.Errorf("parsing token: unexpected nbf: %v", c["nbf"])
	}
	if exp, ok := c["exp"].(float64); !ok || time.Unix(int64(exp), 0).Sub(now) < 0 {
		return fmt.Errorf("parsing token: unexpected exp: %v", c["exp"])
	}
	if sub, ok := c["sub"].(string); !ok || sub == "" || len(sub) > 128 {
		return fmt.Errorf("parsing token: unexpected subject: %q", c["sub"])
	}
	return nil
}
