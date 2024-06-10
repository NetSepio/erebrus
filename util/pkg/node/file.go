package node

import (
	"fmt"
	"net"
	"os"
	"runtime"
)

var (
	osInfo OSInfo
	ipInfo IPInfo
)

func init() {
	osInfo = OSInfo{
		Name:         runtime.GOOS,
		Architecture: runtime.GOARCH,
		NumCPU:       runtime.NumCPU(),
	}

	hostname, err := os.Hostname()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	osInfo.Hostname = hostname

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				ipInfo.IPv4Addresses = append(ipInfo.IPv4Addresses, ipNet.IP.String())
			} else if ipNet.IP.To16() != nil {
				ipInfo.IPv6Addresses = append(ipInfo.IPv6Addresses, ipNet.IP.String())
			}
		}
	}
}

func GetOSInfo() OSInfo {
	return osInfo
}

func GetIPInfo() IPInfo {
	return ipInfo
}
