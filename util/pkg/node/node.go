package node

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/NetSepio/erebrus/core"
	"github.com/NetSepio/erebrus/util/pkg/speedtest"
	"github.com/sirupsen/logrus"
)

// type NodeStatus struct {
// 	Id             string  `json:"id"`
// 	HttpPort       string  `json:"httpPort"`
// 	Domain         string  `json:"domain"`
// 	Address        string  `json:"address"`
// 	Region         string  `json:"region"`
// 	NodeName       string  `json:"nodename"`
// 	DownloadSpeed  float64 `json:"downloadSpeed"`
// 	UploadSpeed    float64 `json:"uploadSpeed"`
// 	StartTimeStamp int64   `json:"startTimeStamp"`
// 	Name           string  `json:"name"`
// 	WalletAddress  string  `json:"walletAddress"`
// 	ChainName      string  `json:"chainName"`
// 	IpInfoIP       string  `json:"ipinfoip"`
// 	IpInfoCity     string  `json:"ipinfocity"`
// 	IpInfoCountry  string  `json:"ipinfocountry"`
// 	IpInfoLocation string  `json:"ipinfolocation"`
// 	IpInfoOrg      string  `json:"ipinfoorg"`
// 	IpInfoPostal   string  `json:"ipinfopostal"`
// 	IpInfoTimezone string  `json:"ipinfotimezone"`
// }

// type NodeStatus struct {
// 	PeerId           string  `json:"peerId" gorm:"primaryKey"`
// 	Name             string  `json:"name"`
// 	HttpPort         string  `json:"httpPort"`
// 	Host             string  `json:"host"` //domain
// 	PeerAddress      string  `json:"peerAddress"`
// 	Region           string  `json:"region"`
// 	Status           string  `json:"status"` // offline 1, online 2, maintainance 3,block 4
// 	DownloadSpeed    float64 `json:"downloadSpeed"`
// 	UploadSpeed      float64 `json:"uploadSpeed"`
// 	RegistrationTime int64   `json:"registrationTime"` //StartTimeStamp
// 	LastPing         int64   `json:"lastPing"`
// 	Chain            string  `json:"chain"`
// 	WalletAddress    string  `json:"walletAddress"`
// 	Version          string  `json:"version"`
// 	CodeHash         string  `json:"codeHash"`
// 	SystemInfo       OSInfo  `json:"systemInfo"`
// 	IpInfo           IPInfo  `json:"ipinfo"`
// }

type NodeStatus struct {
	PeerId           string  `json:"peerId" gorm:"primaryKey"`
	Name             string  `json:"name"`
	HttpPort         string  `json:"httpPort"`
	Host             string  `json:"host"` //domain
	PeerAddress      string  `json:"peerAddress"`
	Region           string  `json:"region"`
	Status           string  `json:"status"` // offline 1, online 2, maintainance 3,block 4
	DownloadSpeed    float64 `json:"downloadSpeed"`
	UploadSpeed      float64 `json:"uploadSpeed"`
	RegistrationTime int64   `json:"registrationTime"` //StartTimeStamp
	LastPing         int64   `json:"lastPing"`
	Chain            string  `json:"chain"`
	WalletAddress    string  `json:"walletAddress"`
	Version          string  `json:"version"`
	CodeHash         string  `json:"codeHash"`
	SystemInfo       string  `json:"systemInfo" gorm:"type:jsonb"`
	IpInfo           string  `json:"ipinfo" gorm:"type:jsonb"`
}

func ToJSON(data interface{}) string {
	bytes, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}
	return string(bytes)
}

// Helper function to convert JSON string to struct
func FromJSON(data string, v interface{}) error {
	return json.Unmarshal([]byte(data), v)
}

type OSInfo struct {
	Name         string // Name of the operating system
	Hostname     string // Hostname of the system
	Architecture string // Architecture of the system
	NumCPU       int    // Number of CPUs
}

type IPInfo struct {
	IPv4Addresses []string
	IPv6Addresses []string
}

func CreateNodeStatus(address string, id string, startTimeStamp int64, name string) *NodeStatus {

	speedtestResult, err := speedtest.GetSpeedtestResults()
	if err != nil {
		logrus.Error("failed to fetch network speed: ", err.Error())
	}
	nodeStatus := &NodeStatus{
		HttpPort:         os.Getenv("HTTP_PORT"),
		Host:             os.Getenv("DOMAIN"),
		PeerAddress:      address,
		Region:           core.GlobalIPInfo.Country,
		PeerId:           id,
		DownloadSpeed:    speedtestResult.DownloadSpeed,
		UploadSpeed:      speedtestResult.UploadSpeed,
		RegistrationTime: startTimeStamp,
		Name:             name,
		WalletAddress:    core.WalletAddress,
		Chain:            os.Getenv("CHAIN_NAME"),
		// IpInfoIP:       core.GlobalIPInfo.IP,
		// IpInfoCity:     core.GlobalIPInfo.City,
		// IpInfoCountry:  core.GlobalIPInfo.Country,
		// IpInfoLocation: core.GlobalIPInfo.Location,
		// IpInfoOrg:      core.GlobalIPInfo.Org,
		// IpInfoPostal:   core.GlobalIPInfo.Postal,
		// IpInfoTimezone: core.GlobalIPInfo.Timezone,
		Version:    "v1",
		CodeHash:   "xxxxxxxxxxxxxxxxxxx",
		SystemInfo: fmt.Sprintf("%+v\n", GetOSInfo()),
		IpInfo:     fmt.Sprintf("%+v\n", GetIPInfo()),
	}

	return nodeStatus
}
