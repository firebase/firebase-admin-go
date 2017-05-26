package auth

import (
	"crypto/rsa"

	"github.com/firebase/firebase-admin-go/internal"
)

var (
	fakePrivateKey = func() *rsa.PrivateKey {
		// Unused private key used solely for testing jwt functionality.
		key, err := internal.ParseKey([]byte(`
-----BEGIN RSA PRIVATE KEY-----
MIIEogIBAAKCAQEAx89EGCCop2dbbjiZOUoyc0IuczSbG5HYl0F2HsZVDqkNya4D
Lh2+rE3Z+VkQkjLqjz83tkjASBWXl/m0kueKId26T1/R8zj5a566a2WH4CBuP0DF
3HIj8+H9Sw1GqQtQn4KbZIA6nOaVBJlamtoKYtZhJddFE6I20LyyvnHQUI24hyMI
G6M3J2jTC9+whZJxvVpmq8AACap5r2T/mLmewremg0PlbQ1+EM5xqqcA5M3jLrHZ
XTKQErA2Ur36ZB2+ssAca59PEbY7zWIiPnTAYsFirSqiY5ag+Oe4LvpTOShdarle
GWnSXk7tE87UeYiK+wHJuOlKVZkzZmIzLwQl0wIDAQABAoIBAFrW1Stu9Z4d9EhY
/Pg5zlPuO7XurbHMDb8+aJg3LQZcP0N4lEOMDFrDjhy5rDn7Yf48DHUYACsFfgT+
5mR/VaJt7r0VYBsGxQZzhGc9Ipf3xoeFSC8fyU6gaIqNf5ls5nuOYl0/muYoQolz
uuh5xo0Gz+XnR6VUcz1U/KJulfl4lcJHicdaMr514EqV15NvwnLvT7MqLL3rBRkn
N0SE7lXV86K6YrR+SA3AGar6Ai2/nhn26YKstiKXeCNN/rIapwrAp9jr7Jz1JHqm
tjfMXxJSiXcM7S6u3H2XeRb+Ry3JZZJazA4C1q1v9Fi6NuCy0eY8TglWcAwtj7yb
et2pOFECgYEA/6C6+duXMw5uMfsBSDxknwiOpH5rkH0quCnYXUKyC6HqwRTMyuaI
sNhuB4cI2SJrUqbcl+jU550w9jwnBRL9rUUmNILyxiFDyorkKFkRAs2GxcEXia6l
XLRrp6lpctFXOTchDU27kEnOz6GCA9mtZco6V77WXYZEBSM/R65jrlkCgYEAyBm7
mHreR5uzoGri8U0CFy3KiqSAqOiVAQG+ot78LtObYoI32pWh3SQZGwpD9GXtggFK
QhFxl915CmYDbs687FdksuQYq3jA27eKNlzL+vnI98LE7sOIB6Eq1owViUpMX8FR
TrNoubI4HZP+RRTvEKnmPtB1ZSl3drV8b7cz6AsCgYBA15SWLI199fsd0n3QxQEB
FjqYnzjJvfZIINUxUum26auSrqQEE9Y4ha3jWu1zprdyj8EFB5p55fW1gCylrNuM
SC4Yw96xQ17e0bxuP6mA/IFjSEegNRzdFyb3sJF+/nsRmFpZ9Y3OW+qJ4H4KW/0Q
BOwntdDKiHRYmUhD9ohygQKBgG3/z2OcL7NXwaAvAgC6X6rUTmJ22g+Ag+Dgz6aD
REiNpP67LO8pkKibnn2B4CdrHOx5vxOguTxN0KtJtxtj5PFbfYzl3TXuFL70H7OQ
wcV/KN4ioNXMgWwISh9VNMWbJW8CO6sy7yAd+8EuyPm670zOyTbAq7hn2jdUv0o+
gPdPAoGAH1inO+h4HngwIH2jASSI3n0bW+ygjEPj8VmwZbmoe7Ga3KQE0Deemkqe
lSJx7yKq+KvNh+1VXfUSc66n4/7D+1ZzBEqDoWbWReEw9eXqzPJaM+PGUgk/dDUa
X+QEGdpIhe1Ga/WO063ujDMW+Nx2I1AJ4icUjiX56mVl0X8Eb40=
-----END RSA PRIVATE KEY-----
`))
		if err != nil {
			panic(err)
		}
		return key
	}()
)
