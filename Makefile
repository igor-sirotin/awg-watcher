APP := awg-watcher
VERSION ?= dev
OPKG_ARCH := mipsel-3.4
ENTWARE_FEED := mipselsf-k3.4
OPKG_FEED_DIR := dist/opkg/$(ENTWARE_FEED)

.PHONY: test lint backend-lint backend-build frontend-test frontend-lint frontend-build build build-linux build-entware-mipsel package-opkg opkg-feed clean

test:
	go test ./cmd/... ./internal/...

lint: backend-lint

backend-lint:
	test -z "$$(gofmt -l $$(find . -path ./frontend -prune -o -path ./.git -prune -o -name '*.go' -print))"
	go vet ./cmd/... ./internal/...

backend-build:
	go build -ldflags "-s -w -X main.version=$(VERSION)" -o bin/$(APP) ./cmd/awg-watcher

frontend/node_modules/.package-lock.json: frontend/package.json frontend/package-lock.json
	cd frontend && npm ci

frontend-test: frontend/node_modules/.package-lock.json
	cd frontend && npm test

frontend-lint: frontend/node_modules/.package-lock.json
	cd frontend && npm run lint

frontend-build: frontend/node_modules/.package-lock.json
	cd frontend && npm run build

build: frontend-build
	go build -ldflags "-s -w -X main.version=$(VERSION)" -o bin/$(APP) ./cmd/awg-watcher

build-linux: frontend-build
	CGO_ENABLED=0 GOOS=linux GOARCH=mipsle GOMIPS=softfloat go build -ldflags "-s -w -X main.version=$(VERSION)" -o bin/$(APP)-mipsle ./cmd/awg-watcher

build-entware-mipsel: build-linux

package-opkg: build-entware-mipsel
	rm -rf $(OPKG_FEED_DIR)
	packaging/opkg/build-ipk.sh $(APP) $(VERSION) $(OPKG_ARCH) bin/$(APP)-mipsle packaging/entware/S97$(APP) $(OPKG_FEED_DIR)

opkg-feed: package-opkg
	packaging/opkg/build-feed.sh $(OPKG_FEED_DIR)

clean:
	rm -rf bin dist frontend/node_modules
