package auth

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
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

var (
	pk = func() *rsa.PrivateKey {
		// Unused private key used solely for testing jwt functionality.
		block, _ := pem.Decode([]byte(`
-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEA20tlytBBq8XceBAW2P1ilb5H12V/XfSqFonrPjCCNc6lWf5E
lchFUGer2EiYWwSJNwGkyYdg7zhoVl0hVmPYSQ7dEi8vXnqDpVivSoIwqmlKdrWr
mZo5CV2p9kgBGxklsf3dstLRxs6eqlwbsx5tD75P5lWJLnBpicrJ1ingBA2KJPIr
DkX1oBqgdVj1hkVztEL9eWNtyBhYNH0lIrUEMdrNAGwSs+BiwxBf6iYkUHb/VDoY
c6mQBJWvOynbpAORGdfxo3yR27ToktTQIVsgT/creFuOcDm/zp9ixQheLa1YOl9T
/fxCAaP/oTMm3wBdgaF8cvgAecEkzhyeE3mtlwIDAQABAoIBAFWrr/m6wF0d8FKL
XpGo89GyQ5i3kzmecrBZcyiZhNUGhPySZDLryYBu7+iP+81bCUwO/VSp6cmrDL/o
pDR+zylDgEQxYN0VGccHzXtbPy3j8m5L7N0WLgAlsld/q8btXRebKPhKeh+j6sJ9
N2kTkuHapJZEhlI5IlHtgkqDk3uhIQ0Msmz2RFTvUeu++uVRPYKuw/i8Am7pDWAf
FEON8dhhPwF8Mxg3RWwsJqitmy8Zslw4JthhY+9PX6HpFOZlMWV3wzRcT52ZOp8Z
jXkxPItePurUbtDCeS9CV1GtBeyrlGKurODG8C93l/Wt3IdfWr6gsWXhholT8hwe
20U47NECgYEA+xfNlRz70UjqilYSnKWtaezhnyPh58cXiE8fhDTuCoYjMtvisfNv
9WbppYK7LnOh0JXBDBDGma2vHQbGeaQK5WclWyVA/7/S4Cj/dO3l57ZUCC099qbS
9mxaRPEoGlDLXbGJpNOlvPBw+47kUTw2qnb6QqJE2oDTppLws4NfBwkCgYEA35SC
czClMDeYU11Eva66d0RK/pw3xToxDbeL61qP+kwlwV7tgBqYTl6BWWWlNwcmI3TJ
Tu7jR3Fk/KOrBC0VzKM3w/n+N9mzxq2GFG6NynT+X9NYnAtXRN9ABXXX+FWxizR2
x540y0t+of2DHcZsxOPDbqnOHGo6x5eFZFFel58CgYEA1yLV6mUi/XZUPqLw33a0
1oU363p7HHPhHdFtV4FiU3IKxpDP81h5HPJITp9scahxhJ5LAWN+Rj4iQ+SCOcbr
7xIpV6bbwkVBEP8PocgTrCz0Yu0goizdpHXCAj/99E41cNmk7azJ3NDGfUM5LMFC
tVuroVwXUn/+2EIeKjDtQsECgYA1ZZ6SLDgHf/+dSVU1iBl4ipLupBidvfwhLoj4
OLTSLoWF3UoTokZl0SRLWX9P2SE+rpG1jFAzq91WiTA62xmtuf2DjJ0ucYwCE0dG
cfDjPXXTJQKwofTBuh/sLezanny8plcH7bzmIK2puoYqAk3P6CWwtFVJbAWFzaZK
AzT4OQKBgHoOaz57080/iZlXW3JfA0jE+66b4VpajTpmzf8LnlyHRpRB11CX2uGw
/6tYHqmsAGubLcHp25sQpE+owdGEEzaFIAO3d4xax53+rKU3mM5P+BDOyq+2H7lJ
2yylhZb9KhqkX5nU4qpNAvDfJEIMuDIBJa03TWNbl9jQc8w+RiYj
-----END RSA PRIVATE KEY-----
`))

		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			panic("Failed to parse private key: " + err.Error())
		}
		return key
	}()
)

// Token represents a decoded Firebase ID token.
//
// Token provides typed accessors to the common JWT fields such as Audience
// (aud) and Expiry (exp). Additionally it provides a UID field, which indicates
// the user ID of the account to which this token belongs. Any additional JWT
// claims can be accessed via the Claims map of Token.
type Token struct {
	Audience  string                 `json:"aud,omitempty"`
	Claims    map[string]interface{} `json:"claims,omitempty"`
	ExpiresAt int64                  `json:"exp,omitempty"`
	IssuedAt  int64                  `json:"iat,omitempty"`
	Issuer    string                 `json:"iss,omitempty"`
	NotBefore int64                  `json:"nbf,omitempty"`
	Subject   string                 `json:"sub,omitempty"`
	UID       string                 `json:"uid,omitempty"`
}

// decodeToken decodes a token and verifies the signature. Requires that the
// token is signed using the RS256 algorithm.
func (c *Client) decodeToken(tokenString string) (*Token, error) {
	rc := rawClaims{}
	token, err := jwt.ParseWithClaims(tokenString, &rc, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("parsing token: invalid signing method: %v", token.Header["alg"])
		}
		return c.kc.get(token.Header["kid"].(string))
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
	for _, k := range reservedClaims {
		delete(rc, k)
	}
	return t, nil
}

func (c *Client) encodeToken(rc rawClaims) (string, error) {
	claims, ok := rc["claims"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("encoding token: invalid claims: %v", rc["claims"])
	}
	if len(claims) == 0 {
		delete(rc, "claims")
	}
	for _, k := range reservedClaims {
		delete(claims, k)
	}
	email := "some@email.com"
	rc["iss"] = email
	rc["sub"] = email
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, rc)
	return token.SignedString(pk)
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
