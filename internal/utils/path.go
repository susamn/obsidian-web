package utils

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidatePath sanitizes and validates a path to prevent directory traversal
// Returns cleaned path and error if invalid
func ValidatePath(path string) (string, error) {
	// Clean the path
	cleanPath := filepath.Clean(path)

	// Handle empty path (root)
	if cleanPath == "." || cleanPath == "" {
		return "", nil
	}

	// Prevent directory traversal
	if strings.Contains(cleanPath, "..") {
		return "", fmt.Errorf("invalid path: directory traversal not allowed")
	}

	// Ensure path doesn't start with /
	cleanPath = strings.TrimPrefix(cleanPath, "/")

	return cleanPath, nil
}

// ValidatePathWithinBase ensures the path is within the base directory
func ValidatePathWithinBase(path, basePath string) (string, error) {
	cleanPath, err := ValidatePath(path)
	if err != nil {
		return "", err
	}

	// Build full path
	fullPath := filepath.Join(basePath, cleanPath)

	// Get absolute paths for comparison
	absBasePath, err := filepath.Abs(basePath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute base path: %w", err)
	}

	absFullPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Ensure requested path is within base directory
	if !strings.HasPrefix(absFullPath, absBasePath) {
		return "", fmt.Errorf("path is outside base directory")
	}

	return cleanPath, nil
}

// IsMarkdownFile checks if a file is a markdown file
func IsMarkdownFile(filename string) bool {
	return strings.HasSuffix(strings.ToLower(filename), ".md")
}
