GO_REPO = github.com/maryvilledev/lxb
GO_INSTALL_PATH = /usr/local/bin/lxb
VERSION = 0.1.0

# the go binary will be named lxb_<os>_<arch>
GO_BIN_NAME = lxb_$$(uname -s -m | tr '[:upper:]' '[:lower:]' | tr ' ' '_')

# Setup the -ldflags option for go build here, interpolate the variable values
LDFLAGS = -ldflags "-X main.AppVersion=$(VERSION)"

# The cgo suffix is for as-true-as-possible static compliation
EXTRAFLAGS = -x -v -a -installsuffix cgo

dist: deps build

build:
	mkdir bin; \
	export CGO_ENABLED=0 GO15VENDOREXPERIMENT=1; \
	go build $(LDFLAGS) $(EXTRAFLAGS) -o bin/$(GO_BIN_NAME)

install: build
	rm -f $(GO_INSTALL_PATH); mkdir -p $(GO_INSTALL_PATH); \
	mv bin/$(GO_BIN_NAME) $(GO_INSTALL_PATH)

test:
	go test -v -bench=.

deps:
	command -v $$GOPATH/bin/glide || go get github.com/Masterminds/glide
	$$GOPATH/bin/glide update
	$$GOPATH/bin/glide install

clean:
	rm -Rf vendor/ glide.lock lxb bin/
