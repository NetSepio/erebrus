package node

import (
	"os"

	"github.com/NetSepio/erebrus/core"
	"github.com/NetSepio/erebrus/util/pkg/speedtest"
	"github.com/sirupsen/logrus"
)

type NodeStatus struct {
	Id             string  `json:"id"`
	HttpPort       string  `json:"httpPort"`
	Domain         string  `json:"domain"`
	Address        string  `json:"address"`
	Region         string  `json:"region"`
	NodeName       string  `json:"nodename"`
	DownloadSpeed  float64 `json:"downloadSpeed"`
	UploadSpeed    float64 `json:"uploadSpeed"`
	StartTimeStamp int64   `json:"startTimeStamp"`
	Name           string  `json:"name"`
	WalletAddress  string  `json:"walletAddress"`
	ChainName      string  `json:"chainName"`
	IpInfoIP       string  `json:"ipinfoip"`
	IpInfoCity     string  `json:"ipinfocity"`
	IpInfoCountry  string  `json:"ipinfocountry"`
	IpInfoLocation string  `json:"ipinfolocation"`
	IpInfoOrg      string  `json:"ipinfoorg"`
	IpInfoPostal   string  `json:"ipinfopostal"`
	IpInfoTimezone string  `json:"ipinfotimezone"`
}

func CreateNodeStatus(address string, id string, startTimeStamp int64, name string) *NodeStatus {
	speedtestResult, err := speedtest.GetSpeedtestResults()
	if err != nil {
		logrus.Error("failed to fetch network speed: ", err.Error())
	}
	nodeStatus := &NodeStatus{
		HttpPort:       os.Getenv("HTTP_PORT"),
		Domain:         os.Getenv("DOMAIN"),
		Address:        address,
		NodeName:       os.Getenv("NODE_NAME"),
		Region:         core.GlobalIPInfo.Country,
		Id:             id,
		DownloadSpeed:  speedtestResult.DownloadSpeed,
		UploadSpeed:    speedtestResult.UploadSpeed,
		StartTimeStamp: startTimeStamp,
		Name:           name,
		WalletAddress:  core.WalletAddress,
		ChainName:      os.Getenv("CHAIN_NAME"),
		IpInfoIP:       core.GlobalIPInfo.IP,
		IpInfoCity:     core.GlobalIPInfo.City,
		IpInfoCountry:  core.GlobalIPInfo.Country,
		IpInfoLocation: core.GlobalIPInfo.Location,
		IpInfoOrg:      core.GlobalIPInfo.Org,
		IpInfoPostal:   core.GlobalIPInfo.Postal,
		IpInfoTimezone: core.GlobalIPInfo.Timezone,
	}
	return nodeStatus
}
