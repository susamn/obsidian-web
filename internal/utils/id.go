package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// GenerateID generates a unique ID for database entries
// Format: timestamp-random
func GenerateID() string {
	timestamp := time.Now().UnixNano()
	randomPart := GenerateRandomString(8)
	return fmt.Sprintf("%d-%s", timestamp, randomPart)
}

// GenerateRandomString generates a random hex string of specified length
func GenerateRandomString(length int) string {
	bytes := make([]byte, length/2)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based if crypto/rand fails
		return fmt.Sprintf("%x", time.Now().UnixNano())[:length]
	}
	return hex.EncodeToString(bytes)
}
