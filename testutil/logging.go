// Copyright (c) 2015 RightScale, Inc. - see LICENSE

package testutil

import (
	"testing"

	"github.com/rs/zerolog"
)

// SetupLogging configures zerolog to discard all logs during tests
func SetupLogging() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}

// RunTests runs the test suite with proper setup and returns the exit code
func RunTests(m *testing.M) int {
	// Set up logging
	SetupLogging()

	// Run tests
	return m.Run()
}
