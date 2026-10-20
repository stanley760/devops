package jwt

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
)

// StaticKeySet is a fixed kid -> public key set, suitable for single-issuer
// deployments or tests. For JWKS rotation, implement the KeySet interface and
// refresh the internal map from a remote JWKS document.
type StaticKeySet struct {
	keys map[string]*rsa.PublicKey
}

// NewStaticKeySet builds a static public key set.
func NewStaticKeySet(keys map[string]*rsa.PublicKey) (*StaticKeySet, error) {
	if len(keys) == 0 {
		return nil, ErrInvalidConfig
	}
	copied := make(map[string]*rsa.PublicKey, len(keys))
	for kid, key := range keys {
		if kid == "" || key == nil {
			return nil, ErrInvalidConfig
		}
		copied[kid] = key
	}
	return &StaticKeySet{keys: copied}, nil
}

// Lookup resolves a public key by kid.
func (s *StaticKeySet) Lookup(kid string) (*rsa.PublicKey, error) {
	if s == nil {
		return nil, ErrInvalidConfig
	}
	key, ok := s.keys[kid]
	if !ok {
		return nil, fmt.Errorf("%w: unknown kid %q", ErrInvalidToken, kid)
	}
	return key, nil
}

// PublicKeyFromPrivatePEM parses the matching public key from a PKCS#1/PKCS#8
// private key PEM, so a service can assemble a self-verifying KeySet from its
// own private key (single-issuer deployment).
func PublicKeyFromPrivatePEM(privatePEM []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(privatePEM)
	if block == nil {
		return nil, ErrInvalidConfig
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		parsed, parseErr := x509.ParsePKCS8PrivateKey(block.Bytes)
		if parseErr != nil {
			return nil, ErrInvalidConfig
		}
		key, ok := parsed.(*rsa.PrivateKey)
		if !ok {
			return nil, ErrInvalidConfig
		}
		return &key.PublicKey, nil
	}
	return &key.PublicKey, nil
}
