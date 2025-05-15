// Copyright (c) 2015 RightScale, Inc. - see LICENSE

package testutil

import (
	"io"
	"testing"

	"gopkg.in/inconshreveable/log15.v2"
)

// SetupLogging configures log15 to discard all logs during tests
func SetupLogging() {
	logWriter := io.Discard // discard logs by default
	log15.Root().SetHandler(log15.StreamHandler(logWriter, log15.TerminalFormat()))
}

// RunTests runs the test suite with proper setup and returns the exit code
func RunTests(m *testing.M) int {
	// Set up logging
	SetupLogging()

	// Run tests
	return m.Run()
}
