# Backward-compatible alias — builds erebrus-node (also installs `erebrus` symlink binary).
# syntax=docker/dockerfile:1
FROM golang:1.26.8-alpine3.23 AS build-app
RUN apk update && apk add --no-cache git
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -tags "with_utls" \
    -ldflags "-X github.com/NetSepio/erebrus/internal/config.Version=2.0.0-$(git rev-parse --short HEAD 2>/dev/null || echo dev)" \
    -o erebrus-node ./cmd/erebrus-node && \
    ln -sf erebrus-node erebrus

FROM alpine:3.23.5
WORKDIR /app
RUN apk update && apk add --no-cache bash wireguard-tools iptables ip6tables bind-tools ca-certificates
COPY --from=build-app /app/erebrus-node .
COPY --from=build-app /app/erebrus .
RUN chmod +x ./erebrus-node ./erebrus

EXPOSE 9080/tcp
EXPOSE 51820/udp
EXPOSE 443/tcp
EXPOSE 443/udp
EXPOSE 9002/tcp

VOLUME ["/var/lib/erebrus", "/etc/wireguard"]

ENTRYPOINT ["/app/erebrus-node"]
