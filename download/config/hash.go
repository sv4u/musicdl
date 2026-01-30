package config

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
)

// ConfigHashLen is the number of hex characters used for the config hash (first 16 of SHA256).
const ConfigHashLen = 16

// HashFromBytes computes the config hash from raw config file bytes.
// Returns the first 16 hex characters of the SHA256 hash.
// Same content always yields the same hash; empty or small input still produces 16 hex chars.
func HashFromBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:ConfigHashLen]
}

// HashFromPath reads the config file at path and returns its hash.
// Uses raw file bytes (no line-ending normalization) so hash matches file content as stored.
func HashFromPath(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return HashFromBytes(data), nil
}
