#!/bin/sh
set -eu

repo="${IPFS_PATH:-/data/ipfs}"
peer_file="$repo/.erebrus-peer-id"
private_file="$repo/.erebrus-private-key"
conflict_file="$repo/.erebrus-identity-conflict"
managed_file="$repo/.erebrus-managed"

expected_peer="$(tr -d '\r\n' <"$peer_file")"
if [ -z "$expected_peer" ]; then
	echo "Erebrus Kubo identity handoff is empty" >&2
	exit 1
fi

actual_peer="$(ipfs config Identity.PeerID)"
if [ "$actual_peer" != "$expected_peer" ]; then
	if [ ! -s "$private_file" ]; then
		printf '%s\n' "expected=$expected_peer actual=$actual_peer" >"$conflict_file"
		echo "Existing Kubo identity conflicts with the Erebrus mnemonic" >&2
		exit 1
	fi
	awk -v peer="$expected_peer" -v private_file="$private_file" '
		BEGIN {
			if ((getline private_key < private_file) < 1) {
				exit 2
			}
			close(private_file)
		}
		{
			sub(/"PeerID": "[^"]*"/, "\"PeerID\": \"" peer "\"")
			sub(/"PrivKey": "[^"]*"/, "\"PrivKey\": \"" private_key "\"")
			print
		}
	' "$repo/config" >"$repo/config.erebrus"
	chmod 600 "$repo/config.erebrus"
	mv "$repo/config.erebrus" "$repo/config"
	if [ "$(ipfs config Identity.PeerID)" != "$expected_peer" ]; then
		echo "Failed to install the Erebrus Kubo identity" >&2
		exit 1
	fi
fi

ipfs config Addresses.API /ip4/0.0.0.0/tcp/5001
ipfs config Addresses.Gateway /ip4/0.0.0.0/tcp/8080
ipfs config --json Gateway.NoFetch true
ipfs config Datastore.StorageMax "${DROP_STORAGE_MAX:-10GB}"
rm -f "$private_file" "$conflict_file"
touch "$managed_file"
