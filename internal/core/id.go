package core

import (
	"crypto/rand"
	"fmt"
)

// GenerateID returns an 8-character lowercase hex string (e.g., "a1b2c3d4").
// Uses crypto/rand for cryptographically secure randomness.
// 4 bytes = ~4.3 billion possibilities, making collisions negligible.
func GenerateID() (string, error) {
	b := make([]byte, 4) // 4 bytes = 8 hex chars
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating id: %w", err)
	}
	return fmt.Sprintf("%x", b), nil
}

// IsUniqueID checks whether the given id is not already present in the existing set.
func IsUniqueID(id string, existing map[string]bool) bool {
	return !existing[id]
}
