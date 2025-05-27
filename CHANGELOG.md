# ChangeLog

## [Unreleased]

### Add

* Add support for per-token passwords for enhanced security (#50)
  - Server: Use `-passwords 'token1:password1,token2:password2'` to require passwords for specific tokens
  - Client: Use `-token 'mytoken:mypassword'` to authenticate with a password

### Fix

* Fix timeout of requests to _token endpoint (#3)

## [v1.1.0](https://github.com/rshade/wstunnel/compare/1.0.7...v1.1.0)

### Add

* Add client parameter -certfile to provide certificate to trust
* Add support for Basic auth in client's tunnel URL ([#60](https://github.com/rshade/wstunnel/issues/60))

### Adding

* adding key lower
* adding in build files ([#3](https://github.com/rshade/wstunnel/issues/3))

### Adding

* Adding in Changelog ([#53](https://github.com/rshade/wstunnel/issues/53))
* Adding Changelog, Changelog Generator, Fixing Client-Ports Usage, Updating to golang 1.14.x ([#39](https://github.com/rshade/wstunnel/issues/39))
* Adding instructions for WSL ([#43](https://github.com/rshade/wstunnel/issues/43))

### Automerge

* automerge ([#51](https://github.com/rshade/wstunnel/issues/51))

### Bump

* Bump github.com/onsi/gomega from 1.35.1 to 1.36.2 ([#42](https://github.com/rshade/wstunnel/issues/42))
* Bump github.com/onsi/gomega from 1.34.2 to 1.35.1 ([#39](https://github.com/rshade/wstunnel/issues/39))
* Bump github.com/onsi/gomega from 1.34.1 to 1.34.2 ([#37](https://github.com/rshade/wstunnel/issues/37))
* Bump github.com/gorilla/websocket from 1.5.1 to 1.5.3 ([#32](https://github.com/rshade/wstunnel/issues/32))
* Bump github.com/onsi/gomega from 1.31.1 to 1.34.1 ([#34](https://github.com/rshade/wstunnel/issues/34))
* Bump golang.org/x/net from 0.20.0 to 0.23.0 ([#30](https://github.com/rshade/wstunnel/issues/30))
* Bump google.golang.org/protobuf from 1.32.0 to 1.33.0 ([#29](https://github.com/rshade/wstunnel/issues/29))
* Bump github.com/onsi/gomega from 1.27.4 to 1.28.0 ([#27](https://github.com/rshade/wstunnel/issues/27))
* Bump gopkg.in/inconshreveable/log15.v2 from 2.0.0-20200109203555-b30bc20e4fd1 to 2.16.0 ([#19](https://github.com/rshade/wstunnel/issues/19))
* Bump github.com/onsi/gomega from 1.24.1 to 1.27.4 ([#22](https://github.com/rshade/wstunnel/issues/22))
* Bump github.com/onsi/gomega from 1.19.0 to 1.24.1 ([#13](https://github.com/rshade/wstunnel/issues/13))
* Bump github.com/onsi/gomega from 1.17.0 to 1.19.0
* Bump github.com/gorilla/websocket from 1.4.2 to 1.5.0
* Bump github.com/onsi/gomega from 1.16.0 to 1.17.0 ([#59](https://github.com/rshade/wstunnel/issues/59))
* Bump github.com/onsi/ginkgo from 1.16.4 to 1.16.5 ([#58](https://github.com/rshade/wstunnel/issues/58))
* Bump github.com/onsi/gomega from 1.15.0 to 1.16.0 ([#57](https://github.com/rshade/wstunnel/issues/57))

### Cmpfusion

* Cmpfusion 176 upgrade websocket golang116 ([#54](https://github.com/rshade/wstunnel/issues/54))

### Configure

* Configure Renovate ([#43](https://github.com/rshade/wstunnel/issues/43))

### Create

* Create go.yml

### Fix

* fix broken WSTunnelClient Stop() ([#52](https://github.com/rshade/wstunnel/issues/52))

### Go

* Go Mod Tidy

### Mapero

* Mapero upstream ([#28](https://github.com/rshade/wstunnel/issues/28))

### Merge

* merge conflicts

### Merge

* Merge branch 'master' into dependabot/go_modules/github.com/gorilla/websocket-1.5.0

### Pinning

* pinning mergo

### Release

* release 1.1.0

### Remove

* remove make depend

### Replace

* replace recently introduced sync.WaitGroup with sync.Cond ([#35](https://github.com/rshade/wstunnel/issues/35)) ([#36](https://github.com/rshade/wstunnel/issues/36))

### Set

* Set InsecureSkipVerify in gorillas websocket Dialer too, if -insecure is set ([#61](https://github.com/rshade/wstunnel/issues/61))

### Update

* Update module github.com/onsi/ginkgo to v2 ([#50](https://github.com/rshade/wstunnel/issues/50))
* Update goreleaser/goreleaser-action action to v6 ([#49](https://github.com/rshade/wstunnel/issues/49))
* Update golang Docker tag to v1.23 ([#48](https://github.com/rshade/wstunnel/issues/48))
* Update actions/setup-go action to v5.3.0 ([#45](https://github.com/rshade/wstunnel/issues/45))
* Update module dario.cat/mergo to v1.0.1 ([#44](https://github.com/rshade/wstunnel/issues/44))

### Updating

* Updating for releases ([#47](https://github.com/rshade/wstunnel/issues/47))
* Updating WSL.md adding sudo ([#44](https://github.com/rshade/wstunnel/issues/44))

### Use

* Use io.Reader to avoid file size limits ([#26](https://github.com/rshade/wstunnel/issues/26))

### Pull Requests

* Merge pull request [#1](https://github.com/rshade/wstunnel/issues/1) from rshade/dependabot/go_modules/github.com/gorilla/websocket-1.5.0
* Merge pull request [#2](https://github.com/rshade/wstunnel/issues/2) from rshade/dependabot/go_modules/github.com/onsi/gomega-1.19.0

## [1.0.7](https://github.com/rshade/wstunnel/compare/1.0.6...1.0.7)

### Add

* Add Support for Wstuncli port range ([#35](https://github.com/rshade/wstunnel/issues/35))
* Add support for binding to the specific host ([#23](https://github.com/rshade/wstunnel/issues/23)) ([#36](https://github.com/rshade/wstunnel/issues/36))
* Add windows support.
* Add ginkgo binary as a dependency
* Add comments to readme

### Build

* Build and Upload, fixes [#34](https://github.com/rshade/wstunnel/issues/34) ([#38](https://github.com/rshade/wstunnel/issues/38))

### Correct

* Correct protocol for tunserv

### Fix

* Fix Travis CI harder
* Fix Makefile to work under Windows/Cygwin
* Fix typo in README
* Fix minor documentation bugs

### Make

* Make GoLint Happy ([#37](https://github.com/rshade/wstunnel/issues/37))
* Make Travis CI happy as well as the Windows build
* Make version.go editing work for Travis

### Merge

* Merge branch 'master' into patch-1

### Removing

* Removing the reference to HTTPS not supported.

### Switch

* Switch from Godep to dep

### Updated

* updated readme for latest version

### Pull Requests

* Merge pull request [#21](https://github.com/rshade/wstunnel/issues/21) from chribben/patch-1
* Merge pull request [#29](https://github.com/rshade/wstunnel/issues/29) from rightscale/ZD156280-1
* Merge pull request [#28](https://github.com/rshade/wstunnel/issues/28) from rightscale/ph-update-dependencies
* Merge pull request [#25](https://github.com/rshade/wstunnel/issues/25) from flaccid/patch-1

## [1.0.6](https://github.com/rshade/wstunnel/compare/1.0.5...1.0.6)

### Add

* add tests to large requests and responses

### Dump

* dump req/resp in case of error, try 2
* dump req/resp in case of error

### Fix

* fix tests not to print to stderr
* fix race condition when reading long messages from websocket in wstuncli
* fix new-websocket race in wstuncli

### Try

* try to fix test race condition [#2](https://github.com/rshade/wstunnel/issues/2)
* try to fix test race condition

### Tweak

* tweak debug output

## [1.0.5](https://github.com/rshade/wstunnel/compare/1.0.4...1.0.5)

### Added

* added logging to track ws connect/disconnect

### Fix

* fix error propagation in wsReader
* fix request timeout calculation
* fix panics when pings fail

### Logging

* logging tweak

### Try

* try to fix random test failures

### Tweak

* tweak readme

### Pull Requests

* Merge pull request [#16](https://github.com/rshade/wstunnel/issues/16) from rightscale/IV-2077_proxy

## [1.0.4](https://github.com/rshade/wstunnel/compare/1.0.3...1.0.4)

### Attempt

* attempt to test websocket reconnection

### Fix

* fix FD leak in wstuncli; add test

### Update

* update readme

## [1.0.3](https://github.com/rshade/wstunnel/compare/1.0.2...1.0.3)

### Fix

* fix non-remove requests on tunnel abort; fix non-reopening of WS at client

## [1.0.2](https://github.com/rshade/wstunnel/compare/1.0.1...1.0.2)

### Added

* added statusfile option

### Readme

* readme fix

### Tweak

* tweak readme

## [1.0.1](https://github.com/rshade/wstunnel/compare/1.0.0...1.0.1)

### Add

* add more debug info to x-host match failure

### Better

* better non-existent tunnel handling

### Change

* change syslog to LogFormatter, which isn't great either

### Fix

* fix recursive code coverage

### Makefile

* makefile fix 4
* makefile fix 3
* makefile fix 2
* makefile fix

### Support

* support syslog on client; improve syslog log format

### Updated

* updated readme

## 1.0.0

### Acu152721

* acu152721 use exec su, since setuid is forced on pre-start
* acu152721 Have upstart use the www-data user

### Add

* add godeps

### Added

* added upstart config files to tgz
* added support for internal servers
* added more tests
* added log15 dependency, added missing test file
* added upload to S3 to makefile, removed binaries from git
* added whois lookup to tunnel endpoint IP addresses
* added stats request; added explicit GC
* added health check; fixed wstunsrv upstart config
* added upstart conf for wstunsrv and minor edit to wstuncli.conf
* added ubuntu upstart and config, license, and binaries

### Created

* created tunnel package

### Delete

* Delete README.md

### First

* first passing test
* first working version

### Fix

* fix cmdline parsing
* fix async AbortConnection
* fix test suite and coverprofile
* fix host header
* fix x-host error responses; add test cases
* fix code coverage badge
* fix travis.yml
* fix go vet booboo
* fix govet complaints
* fix version
* fix readme
* fix some more logging
* fix some more logging
* fix whois regexp
* fix deadlock in stats handler; add reverse DNS lookup for tunnels
* fix problems with chunked encoding and other transfer encodings
* fix problems with chunked encoding and other transfer encodings
* fix readme headings

### Fixed

* fixed FD leak; added syslog flag; added tunnel keys; sanitize tokens from log
* fixed README instructions

### Godep

* godep hell

### Improved

* improved test for host header
* improved logging; removed read/write timeouts due to bug

### Initial

* Initial commit

### Made

* made x-host header work

### Makefile

* makefile tweak
* makefile tweak
* makefile tweak
* makefile tweak

### Merge

* Merge branch 'master' of github.com:rightscale/wstunnel
* Merge branch 'master' of github.com:rightscale/wstunnel
* Merge branch 'master' of github.com:rightscale/wstunnel

### Merge

* merge client & server into a single main

### More

* more debugging for wstuncli errors; inadvertant gofmt

### More

* More readme changes

### Remove

* remove wstunsrv binary again
* remove read/write timeouts on socket; fix logging typo

### Removed

* Removed logfile and added syslog options

### Removed

* removed default for -server option

### Restrict

* restrict _stats to localhost; improve whois parsing; add minimum token length; print error on failed tunnel connect

### Run

* run gofmt

### Squash

* squash file descriptor leak in wstunsrv

### Start

* start on runlevel 2 as well

### Started

* started to update readme

### Timeout

* timeout tweaks, recompile to get go1.3 timeout fixes

### Travis

* travis support

### Update

* Update README.md

### Updated

* updated README

### Version

* version and logging tweaks

### Pull Requests

* Merge pull request [#12](https://github.com/rshade/wstunnel/issues/12) from rightscale/multihost
* Merge pull request [#4](https://github.com/rshade/wstunnel/issues/4) from rightscale/acu153476_use_syslog
* Merge pull request [#2](https://github.com/rshade/wstunnel/issues/2) from rightscale/acu152721_run_as_www_data_user

