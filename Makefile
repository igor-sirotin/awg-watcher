APP := amnezia-config-watch
VERSION ?= dev

.PHONY: test frontend-build build build-linux clean

test:
	go test ./...

frontend/node_modules/.package-lock.json: frontend/package.json frontend/package-lock.json
	cd frontend && npm ci

frontend-build: frontend/node_modules/.package-lock.json
	cd frontend && npm run build

build: frontend-build
	go build -ldflags "-s -w -X main.version=$(VERSION)" -o bin/$(APP) ./cmd/amnezia-config-watch

build-linux: frontend-build
	GOOS=linux GOARCH=mipsle GOMIPS=softfloat go build -ldflags "-s -w -X main.version=$(VERSION)" -o bin/$(APP)-mipsle ./cmd/amnezia-config-watch

clean:
	rm -rf bin frontend/node_modules
