VERSION = $(shell godzil show-version)
CURRENT_REVISION = $(shell git rev-parse --short HEAD)
BUILD_LDFLAGS = "-s -w -X github.com/Songmu/ecschedule.revision=$(CURRENT_REVISION)"
ifdef update
  u=-u
endif

export GO111MODULE=on

.PHONY: deps
deps:
	go get ${u} -d
	go mod tidy

.PHONY: devel-deps
devel-deps: deps
	sh -c '\
      tmpdir=$$(mktemp -d); \
      cd $$tmpdir; \
      go get ${u} \
        golang.org/x/lint/golint            \
        github.com/Songmu/godzil/cmd/godzil \
        github.com/tcnksm/ghr; \
      rm -rf $$tmpdir'

.PHONY: test
test:
	go test

.PHONY: lint
lint: devel-deps
	golint -set_exit_status

.PHONY: build
build: deps
	go build -ldflags=$(BUILD_LDFLAGS) ./cmd/ecschedule

.PHONY: install
install:
	go install -ldflags=$(BUILD_LDFLAGS) ./cmd/ecschedule

.PHONY: release
release: devel-deps
	godzil release

CREDITS: go.sum devel-deps
	godzil credits -w

.PHONY: crossbuild
crossbuild: CREDITS
	godzil crossbuild -pv=v$(VERSION) -build-ldflags=$(BUILD_LDFLAGS) \
      -os=linux,darwin -d=./dist/v$(VERSION) ./cmd/*

.PHONY: upload
upload:
	ghr v$(VERSION) dist/v$(VERSION)
