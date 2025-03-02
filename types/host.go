package types

import (
    "github.com/libp2p/go-libp2p/core/host"
)

var LibP2PHost host.Host

func SetHost(h host.Host) {
    LibP2PHost = h
}

func GetHost() host.Host {
    return LibP2PHost
}