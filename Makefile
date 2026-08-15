VERSION ?= $(shell git describe --tags --always --dirty || echo v0.7.0-dev)
LDFLAGS := -ldflags "-X github.com/cryskram/relith/internal/cli.Version=$(VERSION)"

.PHONY: build build-all run test fmt lint vet tidy clean sqlc release coverage

build:
	go build $(LDFLAGS) ./...

build-all:
	-mkdir bin
	go build $(LDFLAGS) -o bin/relith$(shell go env GOEXE) ./cmd/relith
	go build $(LDFLAGS) -o bin/relithd$(shell go env GOEXE) ./cmd/relithd
	go build $(LDFLAGS) -o bin/relithmcp$(shell go env GOEXE) ./cmd/relithmcp

release:
	goreleaser release --clean

run:
	go run $(LDFLAGS) ./cmd/relithd

test:
	go test -v -race -count=1 ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

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
