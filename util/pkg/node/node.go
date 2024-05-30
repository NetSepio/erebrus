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
	DownloadSpeed  float64 `json:"downloadSpeed"`
	UploadSpeed    float64 `json:"uploadSpeed"`
	StartTimeStamp int64   `json:"startTimeStamp"`
	Name           string  `json:"name"`
	walletAddress  string  `json:"walletAddress"`
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
		Region:         os.Getenv("REGION"),
		Id:             id,
		DownloadSpeed:  speedtestResult.DownloadSpeed,
		UploadSpeed:    speedtestResult.UploadSpeed,
		StartTimeStamp: startTimeStamp,
		Name:           name,
		walletAddress:  core.WalletAddress,
	}
	return nodeStatus
}
