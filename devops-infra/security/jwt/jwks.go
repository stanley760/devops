package jwt

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
)

// JWKSDocument produces an RFC 7517 JWKS document ({"keys":[...]}) for the
// Higress jwt-auth plugin (jwks or jwksFromUrl) and for RS256 verification
// in each service. It emits public key material only (n/e) and never exposes
// the private key.
func JWKSDocument(pub *rsa.PublicKey, keyID string) (map[string]any, error) {
	if pub == nil || pub.N == nil || pub.E <= 0 {
		return nil, fmt.Errorf("%w: missing public key", ErrInvalidConfig)
	}
	if keyID == "" {
		return nil, fmt.Errorf("%w: missing key id", ErrInvalidConfig)
	}
	key := map[string]any{
		"kty": "RSA",
		"kid": keyID,
		"use": "sig",
		"alg": "RS256",
		"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
	}
	return map[string]any{"keys": []any{key}}, nil
}

// JWKS returns the JWKS document for the manager's signing key.
func (m *Manager) JWKS() (map[string]any, error) {
	if m == nil || m.privateKey == nil {
		return nil, ErrInvalidConfig
	}
	doc, err := JWKSDocument(&m.privateKey.PublicKey, m.config.KeyID)
	if err != nil {
		return nil, err
	}
	// Confirm the document marshals cleanly; callers hand it straight to the HTTP layer.
	if _, err := json.Marshal(doc); err != nil {
		return nil, err
	}
	return doc, nil
}
