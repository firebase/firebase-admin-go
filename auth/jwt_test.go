package auth

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func TestEncodeToken(t *testing.T) {
	h := defaultHeader()
	p := mockIDTokenPayload{"key": "value"}
	s, err := encodeToken(&mockSigner{}, h, p)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		t.Errorf("encodeToken() = %d; want: %d", len(parts), 3)
	}

	var header jwtHeader
	if err := decode(parts[0], &header); err != nil {
		t.Fatal(err)
	} else if h != header {
		t.Errorf("decode(header) = %v; want = %v", header, h)
	}

	payload := make(mockIDTokenPayload)
	if err := decode(parts[1], &payload); err != nil {
		t.Fatal(err)
	} else if len(payload) != 1 || payload["key"] != "value" {
		t.Errorf("decode(payload) = %v; want = %v", payload, p)
	}

	if sig, err := base64.RawURLEncoding.DecodeString(parts[2]); err != nil {
		t.Fatal(err)
	} else if string(sig) != "signature" {
		t.Errorf("decode(signature) = %q; want = %q", string(sig), "signature")
	}
}

func TestEncodeSignError(t *testing.T) {
	h := defaultHeader()
	p := mockIDTokenPayload{"key": "value"}
	signer := &mockSigner{
		err: errors.New("sign error"),
	}
	if s, err := encodeToken(signer, h, p); s != "" || err == nil {
		t.Errorf("encodeToken() = (%v, %v); want = ('', error)", s, err)
	}
}

func TestEncodeInvalidPayload(t *testing.T) {
	h := defaultHeader()
	p := mockIDTokenPayload{"key": func() {}}
	if s, err := encodeToken(&mockSigner{}, h, p); s != "" || err == nil {
		t.Errorf("encodeToken() = (%v, %v); want = ('', error)", s, err)
	}
}

type mockSigner struct {
	err error
}

func (s *mockSigner) Email() (string, error) {
	return "", nil
}

func (s *mockSigner) Sign(b []byte) ([]byte, error) {
	if s.err != nil {
		return nil, s.err
	}
	return []byte("signature"), nil
}
