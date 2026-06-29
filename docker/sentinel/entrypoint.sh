#!/usr/bin/env sh
set -e

mkdir -p /etc/unbound/conf.d/generated
mkdir -p /var/lib/erebrus-sentinel
chown -R unbound:unbound /etc/unbound/conf.d/generated /var/lib/erebrus-sentinel 2>/dev/null || true

if command -v unbound >/dev/null 2>&1; then
  unbound -d -c /etc/unbound/unbound.conf &
fi

exec /usr/local/bin/erebrus-sentinel