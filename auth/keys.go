package auth

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"
)

const (
	googleCertURL = "https://www.googleapis.com/robot/v1/metadata/x509/securetoken@system.gserviceaccount.com"
)

var (
	// ErrKeyNotFound is returned when a key is not found.
	ErrKeyNotFound = errors.New("key not found")
)

type keyCache struct {
	sync.Mutex

	keys map[string]*rsa.PublicKey
	exp  time.Time
	hc   *http.Client
}

func (k *keyCache) Get(kid string) (*rsa.PublicKey, error) {
	k.Lock()
	defer k.Unlock()

	now := timeNow()

	if k.keys != nil && k.exp.Sub(now) > 0 {
		if key, ok := k.keys[kid]; ok {
			return key, nil
		}
		return nil, ErrKeyNotFound
	}

	req, err := http.NewRequest(http.MethodGet, googleCertURL, nil)
	if err != nil {
		return nil, fmt.Errorf("generating request: %v", err)
	}

	resp, err := k.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("performing request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("reading keys response body: %v, status code: %d", err, resp.StatusCode)
		}
		return nil, fmt.Errorf("making keys request: response body: %q, status code: %d", resp.StatusCode, body)
	}

	exp := now
	e := resp.Header["Expires"]
	if len(e) > 0 {
		if exp, err = time.Parse(time.RFC1123, e[0]); err != nil {
			return nil, fmt.Errorf("parsing keys expire time: %v", err)
		}
	}

	ks := map[string][]byte{}
	if err = json.NewDecoder(resp.Body).Decode(&ks); err != nil {
		return nil, fmt.Errorf("parsing keys response: %v", err)
	}

	keys := map[string]*rsa.PublicKey{}
	for kid, pk := range ks {
		b, _ := pem.Decode(pk)
		if b == nil {
			return nil, fmt.Errorf("parsing keys: not a pem public key")
		}
		cert, err := x509.ParseCertificate(b.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parsing keys: %v", err)
		}
		pk, ok := cert.PublicKey.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("parsing keys: key is not an RSA public key")
		}
		keys[kid] = pk
	}

	k.exp = exp
	k.keys = keys

	if key, ok := k.keys[kid]; ok {
		return key, nil
	}
	return nil, ErrKeyNotFound
}
