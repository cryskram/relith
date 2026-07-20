VERSION ?= $(shell git describe --tags --always --dirty || echo v0.2.0-dev)
LDFLAGS := -ldflags "-X github.com/cryskram/relith/internal/cli.Version=$(VERSION)"

.PHONY: build build-all run test fmt lint vet tidy clean sqlc release

build:
	go build $(LDFLAGS) ./...

build-all:
	-mkdir bin
	go build $(LDFLAGS) -o bin/relith$(shell go env GOEXE) ./cmd/relith
	go build $(LDFLAGS) -o bin/relithd$(shell go env GOEXE) ./cmd/relithd

release: clean build-all
	@echo "Binaries in bin/"

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
	rm -f coverage.out

coverage:
	go test -race -count=1 -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html