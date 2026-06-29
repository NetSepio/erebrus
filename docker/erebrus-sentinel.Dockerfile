# syntax=docker/dockerfile:1

FROM golang:alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o erebrus-sentinel ./cmd/erebrus-sentinel

FROM alpine:latest
RUN apk add --no-cache ca-certificates wget unbound
WORKDIR /app
COPY --from=build /app/erebrus-sentinel /usr/local/bin/erebrus-sentinel
COPY docker/sentinel/unbound.conf /etc/unbound/unbound.conf
COPY docker/sentinel/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh && \
    mkdir -p /etc/unbound/conf.d/generated /var/lib/erebrus-sentinel && \
    chown -R unbound:unbound /etc/unbound/conf.d/generated /var/lib/erebrus-sentinel

EXPOSE 53/tcp
EXPOSE 53/udp
EXPOSE 8788/tcp

ENV SENTINEL_CONF_DIR=/etc/unbound/conf.d/generated
ENV SENTINEL_LICENSED=true

ENTRYPOINT ["/entrypoint.sh"]