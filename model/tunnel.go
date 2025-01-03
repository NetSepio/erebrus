package model

type Tunnel struct {
	Name      string `json:"name"`
	IpAddress string `json:"IPAddress,omitempty"`
	Port      string `json:"port"`
	Domain    string `json:"domain"`
	Status    string `json:"status,omitempty"`
	CreatedAt string `json:"createdAt"`
}

type Tunnels struct {
	Tunnels []Tunnel `json:"tunnels"`
}
