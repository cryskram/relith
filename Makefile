.PHONY: build run test fmt lint clean sqlc

build:
	go build ./...

run:
	go run ./cmd/cogniqd

test:
	go test ./...

fmt:
	go fmt ./...

sqlc:
	sqlc generate

clean:
	go clean