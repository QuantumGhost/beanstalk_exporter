# some variables
PROGRAM_NAME := beanstalk_exporter
BIN_DIR := bin
DIST_DIR := dist

# build version info
VERSION := v0.0.1
BUILD_DATE := $(shell date -u "+%FT%T+00:00")
COMMIT_SHA1 := $(shell git rev-parse HEAD)

### Golang enviroments and build args
# target info
GOOS := linux
GOARCH := amd64
# Golang enviroments
GOPATH := $(CURDIR)
GOBIN := $(CURDIR)/$(BIN_DIR)
# golang build args
GO_LDFLAGS := -X main.VERSION=$(VERSION) -X main.BUILD_DATE=$(BUILD_DATE) -X \
	main.COMMIT_SHA1=$(COMMIT_SHA1)

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
	@go build -ldflags "$(GO_LDFLAGS)" $(PROGRAM_NAME)

package: install
	@tar -cvzf $(DIST_DIR)/$(PROGRAM_NAME)-$(VERSION).$(GOOS)-$(GOARCH).tar.gz \
	$(BIN_DIR)/$(PROGRAM_NAME)
	$(MAKE) clean

run:
	@go run -ldflags "$(GO_LDFLAGS)" "$(CURDIR)/src/$(PROGRAM_NAME)/main.go"

lint:
	@golint

test:
	@go test
