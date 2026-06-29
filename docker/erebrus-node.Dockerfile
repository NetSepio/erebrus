# syntax=docker/dockerfile:1

FROM golang:alpine AS build-app
RUN apk update && apk add --no-cache git
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -tags "with_reality_server" \
    -ldflags "-X github.com/NetSepio/erebrus/internal/config.Version=2.0.0-$(git rev-parse --short HEAD 2>/dev/null || echo dev)" \
    -o erebrus-node ./cmd/erebrus-node && \
    cp erebrus-node erebrus

FROM alpine:latest
WORKDIR /app
RUN apk update && apk add --no-cache bash wireguard-tools iptables ip6tables bind-tools ca-certificates
COPY --from=build-app /app/erebrus-node .
COPY --from=build-app /app/erebrus .
RUN chmod +x ./erebrus-node ./erebrus

EXPOSE 9080/tcp
EXPOSE 51820/udp
EXPOSE 8443/tcp
EXPOSE 4443/udp

VOLUME ["/var/lib/erebrus", "/etc/wireguard"]

ENTRYPOINT ["/app/erebrus-node"]