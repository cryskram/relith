VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0-dev")
LDFLAGS := -ldflags "-X github.com/cryskram/relith/internal/cli.Version=$(VERSION)"

.PHONY: build run test fmt lint vet tidy clean sqlc

build:
	go build $(LDFLAGS) ./...

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