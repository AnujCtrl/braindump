package core

import (
	"crypto/rand"
	"fmt"
)

// GenerateID returns a 6-character lowercase hex string (e.g., "a1b2c3").
// Uses crypto/rand for cryptographically secure randomness.
func GenerateID() (string, error) {
	b := make([]byte, 3) // 3 bytes = 6 hex chars
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating id: %w", err)
	}
	return fmt.Sprintf("%x", b), nil
}

// IsUniqueID checks whether the given id is not already present in the existing set.
func IsUniqueID(id string, existing map[string]bool) bool {
	return !existing[id]
}
