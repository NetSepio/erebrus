#!/usr/bin/env sh
set -e

mkdir -p /etc/unbound/conf.d/generated
mkdir -p /var/lib/erebrus-sentinel

# Unbound is optional in dev images; sentinel API runs standalone until full DNS stack ships.
if command -v unbound >/dev/null 2>&1; then
  unbound -d -c /etc/unbound/unbound.conf 2>/dev/null &
fi

exec /usr/local/bin/erebrus-sentinel