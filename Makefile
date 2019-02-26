# full pkg name
PKG = github.com/G-Node/gin-cli

# Binary
GIN = gin

# Build loc
BUILDLOC = build

# Install location
INSTLOC = $(GOPATH)/bin

# tests submodule bin
TESTBINLOC = tests/bin

# Build flags
VERNUM = $(shell cut -d= -f2 version)
ncommits = $(shell git rev-list --count HEAD)
BUILDNUM = $(shell printf '%06d' $(ncommits))
COMMITHASH = $(shell git rev-parse HEAD)
LDFLAGS = -ldflags="-X main.gincliversion=$(VERNUM) -X main.build=$(BUILDNUM) -X main.commit=$(COMMITHASH)"

SOURCES = $(shell find . -type f -iname "*.go") version

.PHONY: gin allplatforms install linux windows macos clean uninstall doc

gin: $(BUILDLOC)/$(GIN)

allplatforms: linux windows macos

install: gin
	install $(BUILDLOC)/$(GIN) $(INSTLOC)/$(GIN)

testbuild: linux
	install $(BUILDLOC)/linux/$(GIN) $(TESTBINLOC)/$(GIN)

linux: $(BUILDLOC)/linux/$(GIN)

windows: $(BUILDLOC)/windows/$(GIN).exe

macos: $(BUILDLOC)/darwin/$(GIN)

clean:
	rm -r $(BUILDLOC)

uninstall:
	rm $(INSTLOC)/$(GIN)

$(BUILDLOC)/$(GIN): $(SOURCES)
	go build $(LDFLAGS) -o $(BUILDLOC)/$(GIN)

$(BUILDLOC)/linux/$(GIN): $(SOURCES)
	GOOS=linux GOARCH=amd64 go build -o $(BUILDLOC)/linux/$(GIN) $(LDFLAGS)

$(BUILDLOC)/windows/$(GIN).exe: $(SOURCES)
	GOOS=windows GOARCH=386 go build -o $(BUILDLOC)/windows/$(GIN).exe $(LDFLAGS)

$(BUILDLOC)/darwin/$(GIN): $(SOURCES)
	GOOS=darwin GOARCH=amd64 go build -o $(BUILDLOC)/darwin/$(GIN) $(LDFLAGS)
