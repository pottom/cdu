NAME := cdu
PACKAGE := github.com/pottom/cdu
CMD := cmd/cdu

# cdu carries its own version, tagged cdu-vX.Y.Z so it is never confused with the gdu
# release tags git inherited from the fork (v1.0.0 … v5.36.1). --match keeps git
# describe on cdu's own tags; --dirty marks a build from a modified tree, so a dev
# binary never claims to be a clean release; the "cdu-" prefix is stripped so the stamp
# reads vX.Y.Z, not cdu-vX.Y.Z. An untagged tree falls through to the -dev string below.
RAW_VERSION := $(shell git describe --tags --match 'cdu-v*' --dirty 2>/dev/null)
VERSION := $(subst cdu-,,$(RAW_VERSION))
# Until the first cdu-v tag is cut, an untagged tree is the version under development:
# a clean "-dev" string rather than a bare commit, so the header reads sensibly. Once
# cdu-v0.1.0 exists, git describe wins and the real version shows.
ifeq ($(VERSION),)
VERSION := v0.1.0-dev
endif

# The synced gdu version is not set here: build/cdu.go's GduVersion is its single
# source, compiled in as the default, so nothing stamps it and there is no second copy
# to drift. The upstream watcher bumps that one file.
GDU_VERSION := $(shell sed -n 's/.*GduVersion = "\([^"]*\)".*/\1/p' build/cdu.go)

DATE := $(shell date +'%Y-%m-%d')
GOBIN := go

# CGO is off everywhere (static, portable binaries); the PGO profile is the engine's,
# inherited from gdu and still valid since cdu reuses pkg/analyze unchanged.
GOFLAGS ?= -trimpath -mod=readonly -pgo=default.pgo
LDFLAGS := -s -w \
	-X '$(PACKAGE)/build.Version=$(VERSION)' \
	-X '$(PACKAGE)/build.User=$(shell id -u -n)' \
	-X '$(PACKAGE)/build.Time=$(shell LC_ALL=en_US.UTF-8 date)'

# Release builds — the cross-compiled matrix, archives, checksums, signing, SBOM and
# the container image — are GoReleaser's job, not this file's. The Makefile keeps the
# development targets: build, run, test, lint, man, bench.

# build puts the runnable, stripped binary at the repo root — where cdu is run from
# during development, never dist/ or a temp dir.
build:
	@echo "Version: $(VERSION) (gdu $(GDU_VERSION))"
	GOFLAGS="$(GOFLAGS)" CGO_ENABLED=0 $(GOBIN) build -ldflags="$(LDFLAGS)" -o ./$(NAME) ./$(CMD)

run:
	$(GOBIN) run ./$(CMD)

test:
	gotestsum

coverage:
	gotestsum -- -race -coverprofile=coverage.txt -covermode=atomic ./...

coverage-html: coverage
	$(GOBIN) tool cover -html=coverage.txt

lint:
	golangci-lint run -c .golangci.yml

# gobench guards the scan engine's performance; it is pkg/analyze's own benchmark,
# untouched by the fork.
gobench:
	$(GOBIN) test -bench=. ./pkg/analyze

$(NAME).1: $(NAME).1.md
	sed 's/{{date}}/$(DATE)/g' $(NAME).1.md > $(NAME).1.date.md
	pandoc $(NAME).1.date.md -s -t man > $(NAME).1
	rm -f $(NAME).1.date.md

man: $(NAME).1

show-man:
	sed 's/{{date}}/$(DATE)/g' $(NAME).1.md > $(NAME).1.date.md
	pandoc $(NAME).1.date.md -s -t man | man -l -
	rm -f $(NAME).1.date.md

clean:
	$(GOBIN) mod tidy
	-rm -f coverage.txt $(NAME).1 ./$(NAME)
	-rm -r test_dir dist

install-dev-dependencies:
	$(GOBIN) install gotest.tools/gotestsum@latest
	$(GOBIN) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.2

.PHONY: build run test coverage coverage-html lint gobench man show-man clean install-dev-dependencies
