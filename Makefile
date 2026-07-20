VERSION ?= $(shell git describe --tags --always --dirty || echo v0.3.3-dev)
LDFLAGS := -ldflags "-X github.com/cryskram/relith/internal/cli.Version=$(VERSION)"

.PHONY: build build-all run test fmt lint vet tidy clean sqlc release

build:
	go build $(LDFLAGS) ./...

build-all:
	-mkdir bin
	go build $(LDFLAGS) -o bin/relith$(shell go env GOEXE) ./cmd/relith
	go build $(LDFLAGS) -o bin/relithd$(shell go env GOEXE) ./cmd/relithd
	go build $(LDFLAGS) -o bin/relithmcp$(shell go env GOEXE) ./cmd/relithmcp

release-all: clean
	@mkdir -p bin
	@for platform in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64; do \
		GOOS=$$(echo $$platform | cut -d/ -f1); \
		GOARCH=$$(echo $$platform | cut -d/ -f2); \
		ext=; [ $$GOOS = windows ] && ext=.exe; \
		echo "Building $$GOOS/$$GOARCH..."; \
		GOOS=$$GOOS GOARCH=$$GOARCH go build $(LDFLAGS) -o bin/relith-$$GOOS-$$GOARCH$$ext ./cmd/relith; \
		GOOS=$$GOOS GOARCH=$$GOARCH go build $(LDFLAGS) -o bin/relithd-$$GOOS-$$GOARCH$$ext ./cmd/relithd; \
		GOOS=$$GOOS GOARCH=$$GOARCH go build $(LDFLAGS) -o bin/relithmcp-$$GOOS-$$GOARCH$$ext ./cmd/relithmcp; \
	done
	@echo "Release binaries in bin/:"; ls -1 bin/

run:
	go run $(LDFLAGS) ./cmd/relithd

test:
	go test -v -race -count=1 ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./... || true

tidy:
	go mod tidy

sqlc:
	sqlc generate

clean:
	go clean
	rm -rf bin
	rm -f coverage.out

coverage:
	go test -race -count=1 -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
