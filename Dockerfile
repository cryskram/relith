FROM golang:1.26-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build \
      -ldflags "-s -w -X github.com/cryskram/relith/internal/cli.Version=${VERSION}" \
      -o /out/relith ./cmd/relith && \
    CGO_ENABLED=0 go build \
      -ldflags "-s -w -X github.com/cryskram/relith/internal/cli.Version=${VERSION}" \
      -o /out/relithd ./cmd/relithd && \
    CGO_ENABLED=0 go build \
      -ldflags "-s -w -X github.com/cryskram/relith/internal/cli.Version=${VERSION}" \
      -o /out/relithmcp ./cmd/relithmcp

FROM alpine:3.20
RUN apk add --no-cache git ca-certificates
COPY --from=build /out/relith /usr/local/bin/relith
COPY --from=build /out/relithd /usr/local/bin/relithd
COPY --from=build /out/relithmcp /usr/local/bin/relithmcp

# relithd is the long-running daemon (REST API + graph UI + file watcher).
# Example: docker run -v "$PWD/.relith":/data -e RELITH_CORE_DATA_DIR=/data relith relithd
ENTRYPOINT ["relith"]
CMD ["--help"]
