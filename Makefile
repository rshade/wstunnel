#! /usr/bin/make
#
# Makefile for Golang projects, v0.9.0
#
# Features:
# - runs go tests recursively, computes code coverage report
# - code coverage ready for travis-ci to upload and produce badges for README.md
# - build for linux/amd64, linux/arm, darwin/amd64, windows/amd64
# - just 'make' builds for local OS/arch
# - produces .tgz/.zip build output
# - bundles *.sh files in ./script subdirectory
# - produces version.go for each build with string in global variable VV, please
#   print this using a --version option in the executable
# - to include the build status and code coverage badge in CI use (replace NAME by what
#   you set $(NAME) to further down, and also replace magnum.travis-ci.com by travis-ci.org for
#   publicly accessible repos [sigh]):
#   [![Build Status](https://magnum.travis-ci.com/rightscale/NAME.svg?token=4Q13wQTY4zqXgU7Edw3B&branch=master)](https://magnum.travis-ci.com/rightscale/NAME
#   ![Code Coverage](https://s3.amazonaws.com/rs-code-coverage/NAME/cc_badge_master.svg)
#
# Top-level targets:
# default: compile the program, you can thus use make && ./NAME -options ...
# build: builds binaries for linux and darwin
# test: runs unit tests recursively and produces code coverage stats and shows them
# travis-test: just runs unit tests recursively
# clean: removes build stuff

#NAME=$(shell basename $$PWD)
NAME=wstunnel
EXE:=$(NAME)$(shell go env GOEXE)
BUCKET=rightscale-binaries
ACL=public-read
# dependencies not vendored because used by build & test process
DEPEND=golang.org/x/tools/cmd/cover github.com/rlmcpherson/s3gof3r/gof3r github.com/git-chglog/git-chglog/cmd/git-chglog
HASDEP := $(shell dep version 2> /dev/null)

TRAVIS_BRANCH?=dev
DATE=$(shell date '+%F %T')
TRAVIS_COMMIT?=$(shell git symbolic-ref HEAD | cut -d"/" -f 3)

# This works around an issue between dep and Cygwin git by using Windows git instead.
ifeq ($(shell go env GOHOSTOS),windows)
  ifeq ($(shell git version | grep windows),)
	export PATH:=$(shell cygpath 'C:\Program Files\Git\cmd'):$(PATH)
  endif
endif

# the default target builds a binary in the top-level dir for whatever the local OS is
default: $(EXE)
$(EXE): *.go
	go build -ldflags "-X 'main.VV=$(NAME)_$(TRAVIS_BRANCH)_$(DATE)_$(TRAVIS_COMMIT)'" -o $(EXE) .

# the standard build produces a "local" executable, a linux tgz, and a darwin (macos) tgz
build: depend $(EXE) build/$(NAME)-linux-amd64.tgz build/$(NAME)-windows-amd64.zip
# build/$(NAME)-darwin-amd64.tgz build/$(NAME)-linux-arm.tgz 

# create a tgz with the binary and any artifacts that are necessary
# note the hack to allow for various GOOS & GOARCH combos, sigh
build/$(NAME)-%.tgz: *.go depend
	rm -rf build/$(NAME)
	mkdir -p build/$(NAME)
	tgt=$*; GOOS=$${tgt%-*} GOARCH=$${tgt#*-} go build -ldflags "-X 'main.VV=$(NAME)_$(TRAVIS_BRANCH)_$(DATE)_$(TRAVIS_COMMIT)'" -o build/$(NAME)/$(NAME) .
	chmod +x build/$(NAME)/$(NAME)
	for d in script init; do if [ -d $$d ]; then cp -r $$d build/$(NAME); fi; done
	if [ "build/*/*.sh" != 'build/*/*.sh' ]; then \
	  sed -i -e "s/BRANCH/$(TRAVIS_BRANCH)/" build/*/*.sh; \
	  chmod +x build/*/*.sh; \
	fi
	tar -zcf $@ -C build ./$(NAME)
	rm -r build/$(NAME)

build/$(NAME)-%.zip: *.go depend
	mkdir -p build/$(NAME)
	tgt=$*; GOOS=$${tgt%-*} GOARCH=$${tgt#*-} go build -ldflags "-X 'main.VV=$(NAME)_$(TRAVIS_BRANCH)_$(DATE)_$(TRAVIS_COMMIT)'" -o build/$(NAME)/$(NAME).exe .
	zip $@ build/$(NAME)/$(NAME).exe
	rm -r build/$(NAME)

# upload assumes you have AWS_ACCESS_KEY_ID and AWS_SECRET_KEY env variables set,
# which happens in the .travis.yml for CI
upload: depend
	@which gof3r >/dev/null || (echo 'Please "go get github.com/rlmcpherson/s3gof3r/gof3r"'; false)
	(cd build; set -ex; \
	  for f in *.tgz; do \
		gof3r put --no-md5 --acl=$(ACL) -b ${BUCKET} -k rsbin/$(NAME)/$(TRAVIS_COMMIT)/$$f <$$f; \
		if [ "$(TRAVIS_PULL_REQUEST)" = "false" ]; then \
		  gof3r put --no-md5 --acl=$(ACL) -b ${BUCKET} -k rsbin/$(NAME)/$(TRAVIS_BRANCH)/$$f <$$f; \
		  re='^([0-9]+\.[0-9]+)\.[0-9]+$$' ;\
		  if [[ "$(TRAVIS_BRANCH)" =~ $$re ]]; then \
			gof3r put --no-md5 --acl=$(ACL) -b ${BUCKET} -k rsbin/$(NAME)/$${BASH_REMATCH[1]}/$$f <$$f; \
		  fi; \
		fi; \
	  done)
	(cd build; set -ex; \
		for f in *.zip; do \
		gof3r put --no-md5 --acl=$(ACL) -b ${BUCKET} -k rsbin/$(NAME)/$(TRAVIS_COMMIT)/$$f <$$f; \
		if [ "$(TRAVIS_PULL_REQUEST)" = "false" ]; then \
		  gof3r put --no-md5 --acl=$(ACL) -b ${BUCKET} -k rsbin/$(NAME)/$(TRAVIS_BRANCH)/$$f <$$f; \
		  re='^([0-9]+\.[0-9]+)\.[0-9]+$$' ;\
		  if [[ "$(TRAVIS_BRANCH)" =~ $$re ]]; then \
			gof3r put --no-md5 --acl=$(ACL) -b ${BUCKET} -k rsbin/$(NAME)/$${BASH_REMATCH[1]}/$$f <$$f; \
		  fi; \
		fi; \
	  done)

# version target is now a no-op since we use ldflags to set VV
version:
	@echo "Version is set via ldflags: $(NAME) $(TRAVIS_BRANCH) - $(DATE) - $(TRAVIS_COMMIT)"

# Installing build dependencies is a bit of a mess. Don't want to spend lots of time in
# Travis doing this. The folllowing just relies on go get no reinstalling when it's already
# there, like your laptop.
depend:
	go mod download

clean:
	rm -f $(EXE) version.go
	rm -rf build/

# gofmt uses the awkward *.go */*.go because gofmt -l . descends into the Godeps workspace
# and then pointlessly complains about bad formatting in imported packages, sigh
lint:
	@if gofmt -l $(shell find . -type f -not -path './.*' -not -path './vendor/*' -not -name 'version.go' -name '*.go') | grep .go; then \
	  echo "^- Repo contains improperly formatted go files; run gofmt -w *.go" && exit 1; \
	  else echo "All .go files formatted correctly"; fi
	go vet ./...
	@if command -v golangci-lint > /dev/null; then \
	  echo "Running golangci-lint..." && \
	  golangci-lint run; \
	else \
	  echo "golangci-lint not found, skipping"; \
	fi
	@if command -v yamllint > /dev/null; then \
	  echo "Running yamllint..." && \
	  yamllint --config-file .yamllint .github/workflows/ && \
	  echo "✓ YAML files passed linting"; \
	else \
	  echo "yamllint not found, skipping. Install with: pip install yamllint"; \
	fi

# Auto-fix YAML files
yamllint-fix:
	@if command -v yamllint > /dev/null; then \
	  echo "Checking YAML files for issues..." && \
	  if ! yamllint .github/workflows/ > /dev/null 2>&1; then \
	    echo "YAML linting issues detected. Note: yamllint doesn't have auto-fix capability." && \
	    echo "Showing issues that need manual fixing:" && \
	    yamllint .github/workflows/; \
	  else \
	    echo "✓ All YAML files are valid"; \
	  fi \
	else \
	  echo "yamllint not found. Install with: pip install yamllint"; \
	fi

travis-test: lint
	go test -race -coverprofile=coverage.txt -covermode=atomic ./...

# running tests with coverage
test: lint depend
	go test -v ./...
	go test -race -coverprofile=coverage.txt -covermode=atomic ./...
	go tool cover -func=coverage.txt

# Changelog tools
CHGLOG_VERSION=v0.15.4
CHGLOG := $(shell which git-chglog)

.PHONY: changelog-deps changelog-init changelog changelog-next

changelog-deps:
	@if [ ! -x "$(CHGLOG)" ]; then \
		echo "Installing git-chglog..." && \
		go install github.com/git-chglog/git-chglog/cmd/git-chglog@$(CHGLOG_VERSION) && \
		echo "git-chglog installed successfully"; \
	fi

changelog-init: changelog-deps
	@if [ ! -f .chglog/config.yml ]; then \
		git-chglog --init; \
	else \
		echo "Changelog config already exists in .chglog/"; \
	fi

changelog: changelog-deps
	@echo "Generating changelog..."
	@$(shell go env GOPATH)/bin/git-chglog -o CHANGELOG.md
# make changelog-next VERSION=v1.0.0
changelog-next:
	@echo "Generating changelog for next release..."
	@$(shell go env GOPATH)/bin/git-chglog --next-tag $(VERSION) -o CHANGELOG.md

# Add changelog as dependency to release if it exists
release: changelog