APP := amnezia-config-watch
VERSION ?= dev

.PHONY: test build build-linux clean

test:
	go test ./...

build:
	go build -ldflags "-s -w -X main.version=$(VERSION)" -o bin/$(APP) ./cmd/amnezia-config-watch

build-linux:
	GOOS=linux GOARCH=mipsle GOMIPS=softfloat go build -ldflags "-s -w -X main.version=$(VERSION)" -o bin/$(APP)-mipsle ./cmd/amnezia-config-watch

clean:
	rm -rf bin
