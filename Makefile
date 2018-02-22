# Binary
GIN = gin

# Build loc
BUILDLOC = build

# Install location
INSTLOC = $(GOPATH)/bin

# tests submodule bin
TESTBINLOC = tests/bin

# Build flags
VERNUM = $(shell grep -o -E '[0-9.]+(dev|beta){0,1}' version)
ncommits = $(shell git rev-list --count HEAD)
BUILDNUM = $(shell printf '%06d' $(ncommits))
COMMITHASH = $(shell git rev-parse HEAD)
LDFLAGS = -ldflags "-X main.gincliversion=$(VERNUM) -X main.build=$(BUILDNUM) -X main.commit=$(COMMITHASH)"

SOURCES = $(shell find . -type f -iname "*.go")

.PHONY: gin allplatforms Install linux windows macos clean uninstall

gin: $(BUILDLOC)/$(GIN)

allplatforms: linux windows macos

install: gin
	install $(BUILDLOC)/$(GIN) $(INSTLOC)/$(GIN)

installtest: gin
	install $(BUILDLOC)/$(GIN) $(TESTBINLOC)/$(GIN)

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
	gox -output=$(BUILDLOC)/linux/$(GIN) -osarch=linux/amd64 $(LDFLAGS)


$(BUILDLOC)/windows/$(GIN).exe: $(SOURCES)
	gox -output=$(BUILDLOC)/windows/$(GIN) -osarch=windows/386 $(LDFLAGS)

$(BUILDLOC)/darwin/$(GIN): $(SOURCES)
	gox -output=$(BUILDLOC)/darwin/$(GIN) -osarch=darwin/amd64 $(LDFLAGS)
