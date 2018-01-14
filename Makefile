NAME := branchbot
PKG := github.com/dantoml/branchbot

PREFIX?=$(shell pwd)
BUILDDIR := ${PREFIX}/build

VERSION := $(shell git describe --tags)
BUILD_NUM?=$(shell git rev-parse --short HEAD)-devel
CTIMEVAR=-X $(PKG)/internal/version.UserVersion=$(VERSION) -X $(PKG)/internal/version.BuildNumber=$(BUILD_NUM)
GO_LDFLAGS=-ldflags "-w $(CTIMEVAR)"
GO_LDFLAGS_STATIC=-ldflags "-w $(CTIMEVAR) -extldflags -static"

GOOSARCHES = darwin/amd64 linux/arm linux/arm64 linux/amd64

.PHONY: build
build: *.go
	@echo "+ $@"
	go build ${GO_LDFLAGS} -o $(NAME) .

define buildrelease
GOOS=$(1) GOARCH=$(2) CGO_ENABLED=0 go build \
	 -o $(BUILDDIR)/release/$(NAME)-$(1)-$(2) \
	 -a -tags "static_build netgo" \
	 -installsuffix netgo ${GO_LDFLAGS_STATIC} .;
md5sum $(BUILDDIR)/release/$(NAME)-$(1)-$(2) > $(BUILDDIR)/release/$(NAME)-$(1)-$(2).md5;
sha256sum $(BUILDDIR)/release/$(NAME)-$(1)-$(2) > $(BUILDDIR)/release/$(NAME)-$(1)-$(2).sha256;
endef

.PHONY: release
release: *.go
	@echo "+ $@"
	$(foreach GOOSARCH,$(GOOSARCHES), $(call buildrelease,$(subst /,,$(dir $(GOOSARCH))),$(notdir $(GOOSARCH))))

.PHONY: docker-release
docker-release: release
	@echo "+ $@"
	docker build -t dantoml/branchbot:${VERSION}-${BUILD_NUM} .
	docker push dantoml/branchbot:${VERSION}-${BUILD_NUM}
	docker build -t dantoml/branchbot:latest .
	docker push dantoml/branchbot:latest
