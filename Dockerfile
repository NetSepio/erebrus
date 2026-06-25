# syntax=docker/dockerfile:1

FROM golang:alpine AS build-app
RUN apk update && apk add --no-cache git
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# with_reality_server enables the sing-box REALITY server used by the VLESS
# stealth carrier; keep in sync with the Makefile.
RUN go build -tags "with_reality_server" \
    -ldflags "-X github.com/NetSepio/erebrus/internal/config.Version=2.0.0-$(git rev-parse --short HEAD 2>/dev/null || echo dev)" \
    -o erebrus ./cmd/erebrus

FROM alpine:latest
WORKDIR /app
RUN apk update && apk add --no-cache bash wireguard-tools iptables ip6tables bind-tools ca-certificates
COPY --from=build-app /app/erebrus .
RUN chmod +x ./erebrus

# HTTP API
EXPOSE 9080/tcp
# WireGuard fast path
EXPOSE 51820/udp
# Stealth carriers: VLESS+REALITY (TCP) and Hysteria2 (UDP/QUIC)
EXPOSE 8443/tcp
EXPOSE 4443/udp

# Node state (SQLite + generated secrets/keys) should be a mounted volume.
VOLUME ["/var/lib/erebrus", "/etc/wireguard"]

ENTRYPOINT ["/app/erebrus"]
