package node

import (
	"os"

	"github.com/NetSepio/erebrus/util/pkg/speedtest"
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
}

func CreateNodeStatus(address string, id string, startTimeStamp int64) *NodeStatus {
	speedtestResult, _ := speedtest.GetSpeedtestResults()
	nodeStatus := &NodeStatus{
		HttpPort:       os.Getenv("HTTP_PORT"),
		Domain:         os.Getenv("DOMAIN"),
		Address:        address,
		Region:         os.Getenv("REGION"),
		Id:             id,
		DownloadSpeed:  speedtestResult.DownloadSpeed,
		UploadSpeed:    speedtestResult.UploadSpeed,
		StartTimeStamp: startTimeStamp,
	}
	return nodeStatus
}
