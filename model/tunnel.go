package model

// struct name service
type Tunnel struct { // name app
	Name      string `json:"name"`
	Type      string `json:"type"`
	IpAddress string `json:"IPAddress,omitempty"`
	Port      string `json:"port"`
	Domain    string `json:"domain"`
	Status    string `json:"status,omitempty"`
	CreatedAt string `json:"createdAt"`
}

// type name services
type Tunnels struct {
	Tunnels []Tunnel `json:"service"`
}
