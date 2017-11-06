# Binary
GIN=gin

# Build loc
BUILDLOC=build

# Install location
INSTLOC=$(GOPATH)/bin/

# Build flags
VERNUM=$(shell grep -o -E '[0-9.]+(dev){0,1}' version)
ncommits=$(shell git rev-list --count HEAD)
BUILDNUM=$(shell printf '%06d' $(ncommits))
COMMITHASH=$(shell git rev-parse HEAD)
LDFLAGS=-ldflags "-X main.gincliversion=$(VERNUM) -X main.build=$(BUILDNUM) -X main.commit=$(COMMITHASH)"

gin:
	go build $(LDFLAGS) -o $(BUILDLOC)/$(GIN)

install: gin
	install $(BUILDLOC)/$(GIN) $(INSTLOC)/$(GIN)

allplatforms: linux windows macos

linux: 
	gox -output=$(BUILDLOC)/linux/$(GIN) -osarch=linux/amd64 $(LDFLAGS)

windows:
	gox -output=$(BUILDLOC)/windows/$(GIN) -osarch=windows/386 $(LDFLAGS)

macos:
	gox -output=$(BUILDLOC)/dawrin/$(GIN) -osarch=darwin/amd64 $(LDFLAGS)

clean:
	rm -r $(BUILDLOC)

uninstall:
	rm $(INSTLOC)/$(GIN)
