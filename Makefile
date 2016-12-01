# some variables
PROGRAM_NAME := beanstalk_exporter
BIN_DIR := bin
DIST_DIR := dist

# build version info
# version are filled by CI when building a release.
VERSION := HEAD
BUILD_DATE := $(shell date -u "+%FT%T+00:00")
REVISION := $(shell git rev-parse HEAD)
GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD)

### Golang enviroments and build args
# target info
# Golang enviroments
GOPATH := $(CURDIR)
GOBIN := $(CURDIR)/$(BIN_DIR)
# golang build args
GO_LDFLAGS := -X main.VERSION=$(VERSION) -X main.BUILD_DATE=$(BUILD_DATE) -X \
	main.REVISION=$(REVISION) -X main.GIT_BRANCH=$(GIT_BRANCH)

export GOPATH
export GOBIN


.PHONY: clean clean-dist install build package run lint test

clean:
	@rm -rf "$(CURDIR)/pkg"
	@rm -f $(BIN_DIR)/beanstalk_exporter

clean-dist:
	@find $(CURDIR)/$(DIST_DIR) -name '*.tar.gz' -delete

install:
	@go install -ldflags "$(GO_LDFLAGS)" $(PROGRAM_NAME)

build:
	@go build -ldflags "$(GO_LDFLAGS)" -o $(BIN_DIR)/$(PROGRAM_NAME) $(PROGRAM_NAME)

package: install
	# GOOS and GOARCH are filled by ci when building a release.
	@tar -czf $(DIST_DIR)/$(PROGRAM_NAME)-$(VERSION).$(GOOS)-$(GOARCH).tar.gz \
	$(BIN_DIR)/$(PROGRAM_NAME) README.md
	$(MAKE) clean

run:
	@go run -ldflags "$(GO_LDFLAGS)" "$(CURDIR)/src/$(PROGRAM_NAME)/main.go" $(ARGS)

lint:
	@golint

test:
	@go test $(PROGRAM_NAME)
