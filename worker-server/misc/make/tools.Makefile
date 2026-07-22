# This makefile should be used to hold functions/variables

# Detect OS
UNAME_S := $(shell uname -s)
IS_WINDOWS :=
ifeq ($(OS),Windows_NT)
    IS_WINDOWS := 1
else ifneq (,$(findstring MINGW,$(UNAME_S)))
    IS_WINDOWS := 1
else ifneq (,$(findstring MSYS,$(UNAME_S)))
    IS_WINDOWS := 1
else ifneq (,$(findstring CYGWIN,$(UNAME_S)))
    IS_WINDOWS := 1
endif

ifeq ($(IS_WINDOWS),1)
    EXE := .exe
    export OSTYPE := windows
else
    EXE :=
    export OSTYPE := $(shell uname -s | tr '[:upper:]' '[:lower:]')
endif

# creates a directory bin.
bin:
	@ mkdir -p $@

# ~~~ Tools ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

# ~~ [ goose ] ~~~ https://github.com/pressly/goose ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

GOOSE := bin/goose$(EXE)
goose: bin/goose$(EXE) ## Installs goose (database migration tool)

bin/goose$(EXE): VERSION := 3.11.0
bin/goose$(EXE): GITHUB  := pressly/goose
bin/goose$(EXE): bin
	@ printf "Install goose... "
	@ if [ "$(IS_WINDOWS)" = "1" ]; then \
		GOBIN="$(CURDIR)/bin" go install github.com/pressly/goose/v3/cmd/goose@v$(VERSION); \
	else \
		curl -Ls https://github.com/$(GITHUB)/releases/download/v$(VERSION)/goose_$(VERSION)_$(OSTYPE)_amd64.tar.gz | tar -zOxf - goose > $@ && chmod +x $@; \
	fi
	@ echo "done."

# ~~ [ migrate ] ~~~ https://github.com/golang-migrate/migrate ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

MIGRATE := bin/migrate$(EXE)
migrate: bin/migrate$(EXE) ## Install migrate (database migration)

bin/migrate$(EXE): VERSION := 4.14.1
bin/migrate$(EXE): GITHUB  := golang-migrate/migrate
bin/migrate$(EXE): bin
	@ printf "Install migrate... "
	@ if [ "$(IS_WINDOWS)" = "1" ]; then \
		GOBIN="$(CURDIR)/bin" go install github.com/golang-migrate/migrate/v4/cmd/migrate@v$(VERSION); \
	else \
		curl -Ls https://github.com/$(GITHUB)/releases/download/v$(VERSION)/migrate.$(OSTYPE)-amd64.tar.gz | tar -zOxf - ./migrate.$(OSTYPE)-amd64 > $@ && chmod +x $@; \
	fi
	@ echo "done."

# ~~ [ air ] ~~~ https://github.com/air-verse/air ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

AIR := bin/air$(EXE)
air: bin/air$(EXE) ## Installs air (go file watcher)

bin/air$(EXE): VERSION := 1.67.1
bin/air$(EXE): GITHUB  := air-verse/air
bin/air$(EXE): bin
	@ printf "Install air... "
	@ if [ "$(IS_WINDOWS)" = "1" ]; then \
		GOBIN="$(CURDIR)/bin" go install github.com/air-verse/air@v$(VERSION); \
	else \
		curl -Ls https://github.com/$(GITHUB)/releases/download/v$(VERSION)/air_$(VERSION)_$(OSTYPE)_amd64.tar.gz | tar -zOxf - air > $@ && chmod +x $@; \
	fi
	@ echo "done."


# ~~ [ gotestsum ] ~~~ https://github.com/gotestyourself/gotestsum ~~~~~~~~~~~~~~~~~~~~~~~

GOTESTSUM := bin/gotestsum$(EXE)
gotestsum: bin/gotestsum$(EXE) ## Installs gotestsum (testing go code)

bin/gotestsum$(EXE): VERSION := 1.6.1
bin/gotestsum$(EXE): GITHUB  := gotestyourself/gotestsum
bin/gotestsum$(EXE): bin
	@ printf "Install gotestsum... "
	@ if [ "$(IS_WINDOWS)" = "1" ]; then \
		GOBIN="$(CURDIR)/bin" go install gotest.tools/gotestsum@v$(VERSION); \
	else \
		curl -Ls https://github.com/$(GITHUB)/releases/download/v$(VERSION)/gotestsum_$(VERSION)_$(OSTYPE)_amd64.tar.gz | tar -zOxf - gotestsum > $@ && chmod +x $@; \
	fi
	@ echo "done."

# ~~ [ tparse ] ~~~ https://github.com/mfridman/tparse ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

TPARSE := bin/tparse$(EXE)
tparse: bin/tparse$(EXE) ## Installs tparse (testing go code)

bin/tparse$(EXE): VERSION := 0.8.3
bin/tparse$(EXE): GITHUB  := mfridman/tparse
bin/tparse$(EXE): bin
	@ printf "Install tparse... "
	@ if [ "$(IS_WINDOWS)" = "1" ]; then \
		GOBIN="$(CURDIR)/bin" go install github.com/mfridman/tparse@v$(VERSION); \
	else \
		curl -Ls https://github.com/$(GITHUB)/releases/download/v$(VERSION)/tparse_$(VERSION)_$(OSTYPE)_x86_64.tar.gz | tar -zOxf - tparse > $@ && chmod +x $@; \
	fi
	@ echo "done."

# ~~ [ mockery ] ~~~ https://github.com/vektra/mockery ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

MOCKERY := bin/mockery$(EXE)
mockery: bin/mockery$(EXE) ## Installs mockery (mocks generation)

bin/mockery$(EXE): VERSION := 2.5.1
bin/mockery$(EXE): GITHUB  := vektra/mockery
bin/mockery$(EXE): bin
	@ printf "Install mockery... "
	@ if [ "$(IS_WINDOWS)" = "1" ]; then \
		GOBIN="$(CURDIR)/bin" go install github.com/vektra/mockery/v2@v$(VERSION); \
	else \
		curl -Ls https://github.com/$(GITHUB)/releases/download/v$(VERSION)/mockery_$(VERSION)_$(OSTYPE)_x86_64.tar.gz | tar -zOxf - mockery > $@ && chmod +x $@; \
	fi
	@ echo "done."

# ~~ [ golangci-lint ] ~~~ https://github.com/golangci/golangci-lint ~~~~~~~~~~~~~~~~~~~~~

GOLANGCI := bin/golangci-lint$(EXE)
golangci-lint: bin/golangci-lint$(EXE) ## Installs golangci-lint (linter)

bin/golangci-lint$(EXE): VERSION := 1.39.0
bin/golangci-lint$(EXE): GITHUB  := golangci/golangci-lint
bin/golangci-lint$(EXE): bin
	@ printf "Install golangci-linter... "
	@ if [ "$(IS_WINDOWS)" = "1" ]; then \
		GOBIN="$(CURDIR)/bin" go install github.com/golangci/golangci-lint/cmd/golangci-lint@v$(VERSION); \
	else \
		curl -Ls https://github.com/$(GITHUB)/releases/download/v$(VERSION)/golangci-lint-$(VERSION)-$(OSTYPE)-amd64.tar.gz | tar -zOxf - golangci-lint-$(VERSION)-$(OSTYPE)-amd64/golangci-lint > $@ && chmod +x $@; \
	fi
	@ echo "done."