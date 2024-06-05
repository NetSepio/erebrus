package node

import (
	"os"

	"github.com/NetSepio/erebrus/core"
	"github.com/NetSepio/erebrus/util/pkg/speedtest"
	"github.com/sirupsen/logrus"
)

type NodeStatus struct {
	Id               string  `json:"id"`
	HttpPort         string  `json:"httpPort"`
	Domain           string  `json:"domain"`
	Address          string  `json:"address"`
	Region           string  `json:"region"`
	NodeName         string  `json:"nodename"`
	DownloadSpeed    float64 `json:"downloadSpeed"`
	UploadSpeed      float64 `json:"uploadSpeed"`
	StartTimeStamp   int64   `json:"startTimeStamp"`
	Name             string  `json:"name"`
	WalletAddress    string  `json:"walletAddress"`
	WalletAddresssol string  `json:"walletAddressSol"`
}

func CreateNodeStatus(address string, id string, startTimeStamp int64, name string) *NodeStatus {
	speedtestResult, err := speedtest.GetSpeedtestResults()
	if err != nil {
		logrus.Error("failed to fetch network speed: ", err.Error())
	}
	nodeStatus := &NodeStatus{
		HttpPort:         os.Getenv("HTTP_PORT"),
		Domain:           os.Getenv("DOMAIN"),
		Address:          address,
		NodeName:         os.Getenv("NODE_NAME"),
		Region:           os.Getenv("REGION"),
		Id:               id,
		DownloadSpeed:    speedtestResult.DownloadSpeed,
		UploadSpeed:      speedtestResult.UploadSpeed,
		StartTimeStamp:   startTimeStamp,
		Name:             name,
		WalletAddress:    core.WalletAddressSui,
		WalletAddresssol: core.WalletAddressSolana,
	}
	return nodeStatus
}
