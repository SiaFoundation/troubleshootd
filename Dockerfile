FROM docker.io/library/golang:1.24 AS builder

WORKDIR /troubleshootd

# Install dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Enable CGO for sqlite3 support
ENV CGO_ENABLED=1

RUN go generate ./...
RUN go build -o bin/ -tags='netgo timetzdata' -trimpath -a -ldflags '-s -w -linkmode external -extldflags "-static"'  ./cmd/troubleshootd

FROM debian:bookworm-slim

LABEL maintainer="The Sia Foundation <info@sia.tech>" \
    org.opencontainers.image.description.vendor="The Sia Foundation" \
    org.opencontainers.image.description="A troubleshooter container - troubleshoot host connection issues" \
    org.opencontainers.image.source="https://github.com/SiaFoundation/host-troubleshoot" \
    org.opencontainers.image.licenses=MIT

# Install ca-certificates
RUN apt update && \
    apt upgrade -y && \
    apt install -y --no-install-recommends ca-certificates

# copy binary and prepare data dir.
COPY --from=builder /troubleshootd/bin/* /usr/bin/

# API port
EXPOSE 8080/tcp

ENTRYPOINT [ "troubleshootd" ]
