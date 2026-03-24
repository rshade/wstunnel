// Copyright (c) 2014 RightScale, Inc. - see LICENSE

package main

import (
	"fmt"
	"net"
	_ "net/http/pprof"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rshade/wstunnel/tunnel"
	"github.com/rshade/wstunnel/whois"
)

// VV is the version string, set at build time using ldflags
var VV string

var logger = zerolog.New(os.Stderr).With().Timestamp().Logger()

func init() { tunnel.SetVV(VV) } // propagate version

func main() {
	if len(os.Args) < 2 {
		logger.Fatal().Msgf("Usage: %s [cli|srv|whois|version] [-options...]", os.Args[0])
	}
	switch os.Args[1] {
	case "cli":
		err := tunnel.NewWSTunnelClient(os.Args[2:]).Start()
		if err != nil {
			logger.Fatal().Err(err).Msg("Failed to start client")
		}
	case "srv":
		tunnel.NewWSTunnelServer(os.Args[2:]).Start(nil)
	case "whois":
		lookupWhois(os.Args[2:])
		os.Exit(0)
	case "version", "-version", "--version":
		fmt.Println(VV)
		os.Exit(0)
	default:
		logger.Fatal().Msgf("Usage: %s [cli|srv|whois|version] [-options...]", os.Args[0])
	}
	<-make(chan struct{})
}

func lookupWhois(args []string) {
	if len(args) != 2 {
		logger.Fatal().Msgf("Usage: %s whois <whois-token> <ip-address>", os.Args[0])
	}
	what := args[1]
	names, err := net.LookupAddr(what)
	if err != nil {
		logger.Error().Str("addr", what).Err(err).Msg("DNS lookup failed")
	} else {
		logger.Info().Str("addr", what).Str("dns", strings.Join(names, ",")).Msg("DNS")
	}
	logger.Info().Str("addr", what).Str("whois", whois.Whois(what, args[0])).Msg("WHOIS")
}
