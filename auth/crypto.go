package auth

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type publicKey struct {
	Kid string
	Key *rsa.PublicKey
}

type clock interface {
	Now() time.Time
}

type systemClock struct{}

func (s systemClock) Now() time.Time {
	return time.Now()
}

type mockClock struct {
	now time.Time
}

func (m *mockClock) Now() time.Time {
	return m.now
}

type keySource interface {
	Keys() ([]*publicKey, error)
}

// httpKeySource fetches RSA public keys from a remote HTTP server, and caches them in
// memory. It also handles cache! invalidation and refresh based on the standard HTTP
// cache-control headers.
type httpKeySource struct {
	KeyURI     string
	HTTPClient *http.Client
	CachedKeys []*publicKey
	ExpiryTime time.Time
	Clock      clock
	Mutex      *sync.Mutex
}

func newHTTPKeySource(uri string) *httpKeySource {
	return &httpKeySource{
		KeyURI: uri,
		Clock:  systemClock{},
		Mutex:  &sync.Mutex{},
	}
}

// Keys returns the RSA Public Keys hosted at this key source's URI. Refreshes the data if
// the cache is stale.
func (k *httpKeySource) Keys() ([]*publicKey, error) {
	k.Mutex.Lock()
	defer k.Mutex.Unlock()
	if len(k.CachedKeys) == 0 || k.hasExpired() {
		err := k.refreshKeys()
		if err != nil && len(k.CachedKeys) == 0 {
			return nil, err
		}
	}
	return k.CachedKeys, nil
}

// hasExpired indicates whether the cache has expired.
func (k *httpKeySource) hasExpired() bool {
	return k.Clock.Now().After(k.ExpiryTime)
}

func (k *httpKeySource) refreshKeys() error {
	k.CachedKeys = nil
	if k.HTTPClient == nil {
		k.HTTPClient = http.DefaultClient
	}
	resp, err := k.HTTPClient.Get(k.KeyURI)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	contents, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	newKeys, err := parsePublicKeys(contents)
	if err != nil {
		return err
	}

	maxAge, err := findMaxAge(resp)
	if err != nil {
		return err
	}

	k.CachedKeys = append([]*publicKey(nil), newKeys...)
	k.ExpiryTime = k.Clock.Now().Add(*maxAge)
	return nil
}

type fileKeySource struct {
	FilePath   string
	CachedKeys []*publicKey
}

func (f *fileKeySource) Keys() ([]*publicKey, error) {
	if f.CachedKeys == nil {
		certs, err := ioutil.ReadFile(f.FilePath)
		if err != nil {
			return nil, err
		}
		f.CachedKeys, err = parsePublicKeys(certs)
		if err != nil {
			return nil, err
		}
	}
	return f.CachedKeys, nil
}

func findMaxAge(resp *http.Response) (*time.Duration, error) {
	cc := resp.Header.Get("cache-control")
	for _, value := range strings.Split(cc, ", ") {
		value = strings.TrimSpace(value)
		if strings.HasPrefix(value, "max-age") {
			sep := strings.Index(value, "=")
			if sep == -1 {
				return nil, errors.New("Malformed cache-control header")
			}
			seconds, err := strconv.ParseInt(value[sep+1:], 10, 64)
			if err != nil {
				return nil, err
			}
			duration := time.Duration(seconds) * time.Second
			return &duration, nil
		}
	}
	return nil, errors.New("Could not find expiry time from HTTP headers")
}

func parsePublicKeys(keys []byte) ([]*publicKey, error) {
	m := make(map[string]string)
	err := json.Unmarshal(keys, &m)
	if err != nil {
		return nil, err
	}

	var result []*publicKey
	for kid, key := range m {
		block, _ := pem.Decode([]byte(key))
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		pk, ok := cert.PublicKey.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("Certificate is not a RSA key")
		}
		result = append(result, &publicKey{kid, pk})
	}
	return result, nil
}

func verifySignature(parts []string, k *publicKey) error {
	content := parts[0] + "." + parts[1]
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return err
	}

	h := sha256.New()
	h.Write([]byte(content))
	return rsa.VerifyPKCS1v15(k.Key, crypto.SHA256, h.Sum(nil), []byte(signature))
}
