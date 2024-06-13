package node

import (
	"fmt"
	"net"
	"os"
	"runtime"

	"github.com/NetSepio/erebrus/core"
)

var (
	osInfo    OSInfo
	ipInfo    IPInfo
	ipGeoData IpGeoAddress
)

func Init() {
	osInfo = OSInfo{
		Name:         runtime.GOOS,
		Architecture: runtime.GOARCH,
		NumCPU:       runtime.NumCPU(),
	}

	ipGeoData = IpGeoAddress{
		IpInfoIP:       core.GlobalIPInfo.IP,
		IpInfoCity:     core.GlobalIPInfo.City,
		IpInfoCountry:  core.GlobalIPInfo.Country,
		IpInfoLocation: core.GlobalIPInfo.Location,
		IpInfoOrg:      core.GlobalIPInfo.Org,
		IpInfoPostal:   core.GlobalIPInfo.Postal,
		IpInfoTimezone: core.GlobalIPInfo.Timezone,
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

func GetIpData() IpGeoAddress {
	return ipGeoData
}
