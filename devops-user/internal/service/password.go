package service

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

// errPasswordTooLong means the password exceeds bcrypt's 72-byte limit.
var errPasswordTooLong = errors.New("password too long for bcrypt")

// bcryptCost is the password hash cost.
//
// ⚠️ Value trade-off (bcrypt's compute doubles with every +1 of cost):
//   - <10: insufficient brute-force resistance on modern GPUs/ASICs,
//     especially dangerous with open C-side registration;
//   - >12: login endpoint P99 rises significantly, and login is a
//     high-frequency path.
//
// 10 is the current trade-off. If the C side has explicit login failure rate
// limiting and account-level lockout, it can be raised to 12.
const bcryptCost = bcrypt.DefaultCost

// hashPassword hashes the password (bcrypt; the salt is generated internally
// by bcrypt and encoded in the result).
func hashPassword(plain string) (string, error) {
	// bcrypt only takes the first 72 bytes, and over-long passwords are
	// silently truncated — rejecting explicitly is safer.
	if len(plain) > 72 {
		return "", errPasswordTooLong
	}
	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

// checkPassword verifies the password.
//
// Uses bcrypt's CompareHashAndPassword (constant-time comparison internally),
// rather than comparing hash strings manually.
func checkPassword(plain, hashed string) bool {
	if len(plain) > 72 {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(plain)) == nil
}
