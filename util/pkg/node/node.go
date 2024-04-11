package node

import "os"

type NodeStatus struct {
	Id       string `json:"id"`
	HttpPort string `json:"httpPort"`
	Domain   string `json:"domain"`
	Address  string `json:"address"`
	Region   string `json:"region"`
}

func CreateNodeStatus(address string, id string) *NodeStatus {
	nodeStatus := &NodeStatus{
		HttpPort: os.Getenv("HTTP_PORT"),
		Domain:   os.Getenv("DOMAIN"),
		Address:  address,
		Region:   os.Getenv("REGION"),
		Id:       id,
	}
	return nodeStatus
}
