package hash // import "firebase.google.com/go/auth/hash"

import (
	"encoding/base64"
	"errors"

	"firebase.google.com/go/internal"
)

type Scrypt struct {
	Key           []byte
	SaltSeparator []byte
	Rounds        int
	MemoryCost    int
}

func (s *Scrypt) Config() (*internal.HashConfig, error) {
	if len(s.Key) == 0 {
		return nil, errors.New("signer key not specified")
	}
	if s.Rounds < 1 || s.Rounds > 8 {
		return nil, errors.New("rounds must be between 1 and 8")
	}
	if s.MemoryCost < 1 || s.MemoryCost > 14 {
		return nil, errors.New("memory cost must be between 1 and 14")
	}
	return &internal.HashConfig{
		HashAlgorithm: "SCRYPT",
		SignerKey:     base64.RawURLEncoding.EncodeToString(s.Key),
		SaltSeparator: base64.RawURLEncoding.EncodeToString(s.SaltSeparator),
		Rounds:        int64(s.Rounds),
		MemoryCost:    int64(s.MemoryCost),
	}, nil
}
