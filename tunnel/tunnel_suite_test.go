// Copyright (c) 2015 RightScale, Inc. - see LICENSE

package tunnel

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rshade/wstunnel/testutil"
)

func TestMain(m *testing.M) {
	// Run tests with shared setup
	exitCode := testutil.RunTests(m)

	// Exit with the same code
	os.Exit(exitCode)
}

func TestTunnel(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Tunnel Suite")
}
