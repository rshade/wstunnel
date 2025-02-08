// Copyright (c) 2015 RightScale, Inc. - see LICENSE

package main

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"gopkg.in/inconshreveable/log15.v2"
)

func TestWstunnel(t *testing.T) {
	// buffer up log messages in ginkgo and only output on error
	log15.Root().SetHandler(log15.StreamHandler(GinkgoWriter, log15.TerminalFormat()))

	format.UseStringerRepresentation = true
	RegisterFailHandler(Fail)
	RunSpecs(t, "WSTUNNEL")
}
