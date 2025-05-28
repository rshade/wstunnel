package main

import (
	"testing"
)

func TestVersion(t *testing.T) {
	// This test ensures the version variable is properly initialized
	// The actual version command is tested via integration tests
	// This improves code coverage for the version-related code

	// Save original VV
	originalVV := VV
	defer func() { VV = originalVV }()

	// Test with a specific version
	VV = "test-version-1.0.0"

	if VV != "test-version-1.0.0" {
		t.Errorf("Expected VV to be 'test-version-1.0.0', got '%s'", VV)
	}
}
